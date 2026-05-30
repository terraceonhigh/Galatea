# STATUS — the cursor

**The single source of "where are we."** First thing a resuming session reads
(see [`DEVELOPMENT-LOOP.md`](DEVELOPMENT-LOOP.md) § Recovery); last thing every
loop updates. If this file and the code disagree, the code is truth — fix this.

---

**Updated:** 2026-05-29 (autonomous run: R1 gated; **R2 complete + its behavioural
gate met** — the lifted server *executes* a COMPOUND{PUTROOTFH,GETATTR} →
NFS4_OK over the in-memory FSAL, in-process; cursor at R3 — serve over the wire +
a real handle resolver)
**Goal:** [`GOAL.md`](GOAL.md) — Milestone A (read-write, Finder-visible
filesystem of our own).
**Build state:** green — `go build ./... && go vet ./... && go test ./...` all
pass; `go fmt` clean. (The mid-run global-hook block is cleared — see
`MISTAKES.md` M-003.)

## Done

- **R0 — FSAL foundation.** `pkg/virtual` (hand-cut, bb-storage-free interface +
  in-memory FSAL), `pkg/osfs` (read-only local-filesystem backend), `cmd/galatea`
  (CLI navigator). Coupling measured ([`coupling-map.md`](coupling-map.md)).
  Decisions DEC-001…DEC-006.
- **R2a — go-xdr vendored.** `internal/xdr/` holds the XDR codec + NFSv4/RPCv2/
  darwin wire stubs, self-contained (stdlib-only), smoke-tested. DEC-010.
- **Serving foundation.** `internal/xdr/pkg/rpcserver` (ONC-RPC loop + AUTH_SYS)
  vendored; `golang.org/x/sync/errgroup` replaced by self-contained
  `internal/errgroup`. Mount feasibility proven (M-004): NetFS/automountd path is
  present, so **(A) is reachable here** — no root needed.
- **R2b — `path`+`filesystem` vendored.** `internal/bb/filesystem/{,path}` —
  copied, grpc-status error idiom stripped to stdlib, `x/sys/unix` dev_t packing
  reimplemented inline (kept the module dependency-free). Builds/vets/tests/fmt
  green standalone; path smoke test guards the strip. DEC-014, `internal/bb/VENDOR.md`.
- **R2c — `pkg/virtual` re-pointed onto the vendored types.** `types.go`'s
  hand-cut leaf types retired, replaced by **aliases** (`Component`=`path.Component`,
  `FileType`=`filesystem.FileType`, …) + const/constructor re-exports;
  `Attributes.symlinkTarget` reverted `string`→`path.Parser`. Aliases → the server
  (R2d) meets the interface with zero type conversion; ~170 use-sites untouched.
  R0 suite re-verified green. DEC-015.
- **R2d — the NFSv4 server lifted.** bb-rex's `nfs40/nfs41_program`,
  `opened_files_pool`, `minor_version_fallback` copied to **`internal/nfsv4`**;
  imports rewritten (`virtual`→`pkg/virtual`, bb-storage→`internal/bb`,
  go-xdr→`internal/xdr`); `system_authenticator` rewritten to localhost AUTH_SYS;
  prometheus stripped; `metrics_program` dropped. `pkg/virtual` gained the symbols
  the lift actually needed (`ByteRangeLock*`, `HandleResolver`, `Format`/
  `UNIXFormat`, `VirtualSymlink`→`Parser`). **`go list` shows zero buildbarn
  imports.** A latent upstream 4.1-SEQUENCE deadlock was caught by `go vet` and
  fixed (M-005). DEC-016, `internal/nfsv4/VENDOR.md`. **R2 structurally complete.**
- **R2 behavioural gate MET (+ R3 in-process COMPOUND).** The lifted server now
  *executes*: `internal/nfsv4.NewReadOnlyProgram(root)` over the in-memory FSAL,
  then `NfsV4Nfsproc4Null` + `NfsV4Nfsproc4Compound{PUTROOTFH, GETATTR}` →
  `NFS4_OK` (`internal/nfsv4/program_test.go`, in-process — no RPC framing). Got
  there via DEC-017 Option B step 1 (in-memory `FileHandle`) + adding
  `IsInNamedAttributeDirectory` to the FSAL (the server's FATTR4 type-encoding
  reads it; surfaced as a panic, fixed). This is the DEC-011/016 smoke gate.

## Cursor — next increment

**R3 — serve NFSv4 over the wire (the in-process COMPOUND already works).**
([`ROADMAP.md`](ROADMAP.md) R3)

> **Done when:** a NULL call + a basic COMPOUND (PUTROOTFH+GETATTR) complete
> against the running server over a **loopback socket** (through
> `rpcserver.HandleConnection` with ONC-RPC record marking) — `pynfs` or a minimal
> in-tree client.
>
> **Already done (in-process):** the re-scoped R2 behavioural gate — the program
> executes the same COMPOUND directly (`internal/nfsv4/program_test.go`). What
> remains for R3 proper is the *RPC framing* layer around it.

**Two remaining pieces for R3:**
1. **Serve over the wire.** Wrap the program with `nfsv4.NewNfs4ProgramService`,
   `rpcserver.NewServer({100003: service}, NewSystemAuthenticator())`, and drive
   `HandleConnection(r, w)` over an in-process `net.Pipe` (smoke) or a loopback
   TCP listener (`cmd/galatea serve`). The smoke client must build a record-marked
   ONC-RPC CALL (xid, prog 100003, vers 4, proc 1, AUTH_SYS cred) wrapping the
   `Compound4args` — the one laborious bit; crib framing from `rpcserver`'s reader.
2. **A real `HandleResolver`** (DEC-017 Option B): the in-memory FSAL keeps a
   `map[handle]Node` (register nodes as they're created/walked) so PUTFH/LOOKUP/
   READ resolve. The current `NewReadOnlyProgram` stub rejects all handles —
   fine for PUTROOTFH+GETATTR, *not* for browsing. Needed before R4 (a real mount
   does LOOKUP/READDIR/READ).

Everything the server needs to run now exists in-tree and green: the COMPOUND
engine (`internal/nfsv4`), the wire codec + ONC-RPC record-marking loop
(`internal/xdr/pkg/{runtime,protocols,rpcserver}`), the localhost AUTH_SYS
authenticator, and two backends to serve (`pkg/virtual` in-memory, `pkg/osfs`).

**Serving API — mapped (R3 investigation, this run):** wiring is small —
`service := nfsv4.NewNfs4ProgramService(program)` yields exactly a
`rpcserver.Service`; `rpcserver.NewServer(map[uint32]Service{nfsv4.NFS4_PROGRAM_`
`PROGRAM_NUMBER (100003): service}, authenticator)`; then `server.HandleConnection`
`(r, w)` per connection. The smoke test can drive `HandleConnection` over an
in-memory pipe — no TCP or pynfs needed for NULL + PUTROOTFH+GETATTR.

**⚠ R3 PREREQUISITE — handle allocation (DEC-017, RESOLVED → Option B).**
`NewNFS40Program` reads the root's `FileHandle` at construction (panics if unset),
and `NewOpenedFilesPool` needs a `virtual.HandleResolver` (handle → node). The R0
backends provide neither. **This run attempted Option A (lift bb-rex's handle
allocator) and rejected it** — it drags `LinkableLeaf`→`InitialNode`→the
`PrepopulatedDirectory` node framework the hand-cut `pkg/virtual` deliberately
omitted (imports looked clean; the symbol cascade wasn't — the R2d lesson again).
**Chosen: Option B — backends self-assign handles** + a small Galatea-written
resolver. Keeps the lightweight node model. See DEC-017.

So the R3 opener is now concrete:
1. ✅ **In-memory FSAL `FileHandle`** — done. `memory.go` self-assigns an 8-byte
   big-endian handle from each node's inode under `AttributesMaskFileHandle`
   (`memoryFileHandle`); tested in `handle_test.go`. This unblocks
   `NewNFS40Program` (which panicked reading the root handle). A map-backed
   `HandleResolver` (handle → node) is still TODO — needed for LOOKUP/PUTFH browse
   (R4); the NULL+PUTROOTFH+GETATTR smoke can use a stub resolver (never called).
2. **A program convenience constructor** in `internal/nfsv4` (fill `NewNFS40Program`'s
   args: `NewOpenedFilesPool(stubResolver)`, `random.NewFastSingleThreadedGenerator()`,
   `clock.SystemClock`, zero verifier/prefix, lease times, `path.UNIXFormat`).
3. **Smoke**: call `program.NfsV4Nfsproc4Null` then `NfsV4Nfsproc4Compound` with a
   hand-built `Compound4args{PUTROOTFH, GETATTR}` **directly** (no rpcserver /
   record-marking needed — that's the cheap path); assert `NFS4_OK`. Then, for the
   full R3 gate, the same over `rpcserver.HandleConnection` via an in-process pipe.

Remaining work for R3 (after the handle decision):
- Resolve DEC-017; give the backend(s) FileHandle + a resolver.
- Add `cmd/galatea serve` / a `pkg`-level `Serve`: build the program over a
  backend, register prog 100003, listen on loopback.
- Drive it: NULL, then PUTROOTFH+GETATTR (in-memory FSAL, in-process pipe).
- Expect macOS-client-quirk discovery to begin (R5 leaking early); journal it.

> **⚠ Do NOT vendor `util` wholesale** — jsonnet/protobuf/grpc/prometheus/uuid.
> (Retained from R2; applies to any future bb-storage symbol grab.)

**After R3:** R4 — `open nfs://localhost:PORT/` to mount read-first. The first
privileged/GUI-adjacent step: M-004 shows the mount path is open without root, but
confirming Finder visibility needs the Architect's eyes on a non-headless Mac —
**the likely first genuinely-blocking wall for a headless agent.**

The server's `virtual.*` surface is entirely interface/attributes/status/
permissions — **all in `pkg/virtual`**, no handle-allocator/CAS types (a big R2d
de-risk, measured in `coupling-map.md`). The caution that governed R2a/R2b still
governs R2d's `clock`/`random` shims:

> **⚠ Do NOT vendor `util` wholesale** — it pulls jsonnet, protobuf, grpc,
> prometheus, uuid. The "8-package floor" (coupling-map) is *symbol*-light but
> *transitive-dependency*-heavy: vendor only the symbols actually used (here,
> nothing from util survives the error-wrapper strip). Same caution for any floor
> package taken whole.

**After R2:** R3 — wire `cmd/galatea serve` (rpcserver loop, NFS prog 100003) + a
smoke COMPOUND test; then R4 — `open nfs://localhost:PORT/` to mount read-first.

Loop step to resume at: **4 (Implement)** for R3 — (1) serve over the wire
(`rpcserver` + `HandleConnection`, record-marked client), (2) a real
`HandleResolver` for the in-memory FSAL. The in-process COMPOUND already passes
(R2 behavioural gate met). (R2 complete — DEC-016; server executes — this run.)

> **Tooling gotchas this run:** (1) `cd` in Bash *persists* the working dir across
> calls and breaks later relative-path commands — use absolute paths / `git -C`.
> (2) The classifier ("…temporarily unavailable") and the global hook intermittently
> block Bash; retry, don't thrash.

### Mount feasibility — CORRECTED (see M-004)

Earlier this run I wrongly called R1/R4 "needs root, insurmountable here." Testing
falsified it:

- `mount_nfs` as uid 501 → *Connection refused* (exit 61), not *Operation not
  permitted*. Root is not the gate at the network phase.
- The **NetFS/`automountd` path is present** (`/usr/bin/open`,
  `/usr/libexec/automountd`, `NetFS.framework`) — the unprivileged mount
  mechanism FUSE-T uses. Plan for R4: `open nfs://localhost:PORT/…` (or the
  NetFSMountURLAsync API), which has `automountd` mount on our behalf — no root.
- **(A) is therefore very likely reachable on this Mac.** Not yet *proven* end to
  end (no server to mount until R3); the mount *path* is open, the full mount +
  Finder display is confirmed at R4.
- Finder visibility is verifiable: the Architect can look, and `mount` / `df` /
  `ls /Volumes` confirm it programmatically (this run is headless, the Mac is not).
- Possible residual ask: if the NetFS path needs one privileged setup, the
  Architect grants a single sudo'd command. Bounded, not a wall.

## Known blocks / open questions for upcoming increments

- **Worktree can't see `references/`** (gitignored). R2's server lift needs them
  to compile. The interface package sidestepped this by being dependency-free;
  the server won't. Options: build from the main checkout, or have the Architect
  create symlinks (`ln` is not on the sandbox allowlist, so the agent can't make
  them itself). Sketched in DEC-004.
- **Mounting needs privileges** (R4). Whether `mount_nfs` / `NetFSMountURLAsync`
  can be driven in this environment without interactive sudo is unconfirmed —
  resolve when R4 begins; may require the Architect's hand or a dev Mac.
- **The type-reconciliation fork** (DEC-005 → DEC-007) is unresolved by design;
  decide it with the server code in front of you at R2.

## Notes for the resuming session

- You are **Daedalus**. Start with the Recovery procedure in
  [`DEVELOPMENT-LOOP.md`](DEVELOPMENT-LOOP.md).
- Mercer (Comprador) was last written at `~/Labs/Comprador/correspondence/17`.
  Don't re-open unless you have something load-bearing; one letter/week at most.
- **Minerve (Stepford) is now a correspondent** — see `Correspondance/`
  02 (her inquiry) and 03 (Daedalus's reply). She is building a no-kext FOSS NTFS
  driver for macOS and intends to ride Galatea as an **FSAL backend** (ntfs-3g,
  recommended out-of-process). This means Galatea now has **two confirmed
  downstream consumers** — Comprador/MTP and Stepford/NTFS — and the FSAL boundary
  should be designed against both, not MTP alone. Her NTFS backend is structurally
  `pkg/osfs` made read-write through ntfs-3g; that backend is her template. The
  ball is in her court (she'll write back); don't initiate. Note for whoever lifts
  the server (R2) and shapes `galatea.Mount`: a second consumer's needs are now on
  record in letter 03.
- This branch is the **canonical Phase-1 line.** A parallel exploratory branch
  (`stoic-zhukovsky`, the replace-directive whole-server lift) was set aside by
  the Architect; don't resurrect or duplicate it. Letters 02/03 were authored
  there and migrated here.
