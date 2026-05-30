# STATUS — the cursor

**The single source of "where are we."** First thing a resuming session reads
(see [`DEVELOPMENT-LOOP.md`](DEVELOPMENT-LOOP.md) § Recovery); last thing every
loop updates. If this file and the code disagree, the code is truth — fix this.

---

**Updated:** 2026-05-29 (autonomous run: **R0→R4 read-only + R6a write path DONE —
Galatea mounts live on macOS, headless, no root, and serves a read-WRITE tree**
(write + append + truncate all land correctly over NFS). The
central thesis is proven. Cursor: R6b (dir mutation + truncate) / R5 (conformance)
/ R1 (timeout). The Finder GUI screenshot is the only Architect-gated bit.)
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
- **R3 — serve NFSv4 over the wire.** `cmd/galatea serve` listens on loopback TCP,
  wraps the program with `nfsv4.NewNfs4ProgramService` + `rpcserver.NewServer` +
  the localhost AUTH_SYS authenticator. NULL + COMPOUND proven over a real TCP
  dial (`internal/nfsv4/wire_test.go: TestServeOverTCP`) and a `net.Pipe`. The
  real `HandleResolver` (DEC-017 Option B) landed too — `virtual.NewMemoryHandleResolver`,
  GETFH/PUTFH handle round-trip tested.
- **R4 (read-only) — LIVE MOUNT, headless, no root.** The macOS kernel NFS client
  `mount_nfs`'d `galatea serve` as uid 501; `ls`/`cat` browsed and read the demo
  tree (README.txt, docs/note.txt) correctly; clean `umount`. Full read path
  (handshake → PUTROOTFH/GETATTR/ACCESS/LOOKUP/GETFH/PUTFH/OPEN/READ). The
  "mounting needs root" assumption is falsified with a live receipt. **The central
  thesis is proven.** DEC-018; M-006 (the one attribute that crashed the first
  attempt, now guarded by `TestMemoryMandatoryAttributes`).
- **R6a — writable in-memory files (read-WRITE over a live mount).** `memoryFile`
  is mutable under a per-file mutex (write/zero-extend/truncate/allocate);
  `TestFileWrite` covers it. **Live-verified:** `printf … > mnt/README.txt`
  (truncating write), `>>` append, and read-back all correct over the macOS NFS
  mount — `> file` yields exactly the new bytes. Galatea now serves a read-write
  filesystem. The earlier truncate-on-`>` glitch was a `VirtualSetAttributes` bug
  (applied the `requested` *return* mask instead of what `in` carried) — found by
  an env-gated COMPOUND op-trace (`GALATEA_TRACE=1`) of the live client, fixed,
  and the test strengthened to catch it.

## Cursor — next increment

**R5 / R6 — conformance and the write path** (Milestone A's remaining gates).
([`ROADMAP.md`](ROADMAP.md))

R0→R4 read-only is done and live (see Done above + DEC-018): Galatea mounts on
macOS, headless, no root, and serves a browsable/readable tree. What remains for
Milestone A:

- **R5 — read-only conformance.** Run the read-applicable `pjdfstest` subset and a
  `pynfs` NFSv4.0 read subset against a live `galatea serve` mount; enumerate
  exclusions; stand up `make test-conformance`. Runnable headless now (mounting
  works; `pjdfstest` is a C suite executed at the mountpoint).
- **R6 — the write path.** *R6a done* (in-memory file contents are writable —
  write/append/truncate proven live). **R6b remaining:** (1) **directory
  mutation** — CREATE/MKDIR/REMOVE/RENAME (the in-memory dir still returns
  `StatusErrROFS`; needs tree-level locking, since creating/removing mutates the
  children map while reads traverse it — the concurrency design R6a deliberately
  deferred); (2) the `pjdfstest` write subset. `osfs` write (mutating the real disk) is a separate,
  later call.
- **R1 — the substrate bet (now runnable).** Measure that a multi-minute slow read
  over the mount does *not* hit the RPC-timeout class that stalled NFSv3: put a
  deliberately-slow backend behind `galatea serve` and time a large read.
- ✅ **`osfs` handles — done.** `osfs` now provides path-relative file handles +
  `NewHandleResolver` + the mandatory attributes (M-006 contract), tested. `galatea
  serve <host-dir> [addr]` mounts a **real host directory** — verified live: served
  the repo's `docs/` over NFSv4, `ls` listed all 7 files and `head GOAL.md` read it
  over the mount, clean `umount`. (Caveat: path handles are bounded by NFS4_FHSIZE
  ≈128 B; deeply-nested paths would need an inode/hash scheme — future.)

Pick R5 or R6 next (R5 hardens what works; R6 extends toward read-write, the
Milestone-A goal). The **one genuinely-deferred item** is a human-eyes Finder GUI
screenshot (the Architect's non-headless Mac); it gates nothing — `ls`/`mount`/`df`
verify the mount programmatically — but it's the satisfying visual confirmation.

> **⚠ Do NOT vendor `util` wholesale** — jsonnet/protobuf/grpc/prometheus/uuid;
> vendor by symbol. (Retained; applies to any future bb-storage grab, e.g. R6.)

Loop step to resume at: **2 (Scope)** for R5 or R6 — choose the gate, restate its
"Done when", then investigate. R0→R4 read-only is banked and green.

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
