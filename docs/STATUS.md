# STATUS — the cursor

**The single source of "where are we."** First thing a resuming session reads
(see [`DEVELOPMENT-LOOP.md`](DEVELOPMENT-LOOP.md) § Recovery); last thing every
loop updates. If this file and the code disagree, the code is truth — fix this.

---

**Updated:** 2026-05-29 (autonomous run: **R0→R4 + R6a + most of R6b DONE — Galatea
is a working read-WRITE NFS filesystem on macOS**, live, headless, no root:
read/write/append/truncate + create/mkdir/rm/rmdir/**rename** all verified over a
real mount (`go test -race` clean). **R1 (the founding substrate bet) is also
validated** — a 2m10s READ completed over NFSv4 where NFSv3 would have timed out.
**R7's AC2 (sustained transfer) is validated too** — a 1 GB payload round-trips
write→server→remount→read byte-for-byte identical. The central thesis is proven
and exceeded. Cursor: R5 (pjdfstest conformance) / osfs-write; Mknod/Link/Symlink
are niche follow-ups. Architect-gated remainders: the Finder GUI screenshot and
R7's sleep-wake half.)
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
- **R7 (AC2 — sustained transfer) — VALIDATED.** A 1 GB random payload `dd`'d onto
  the writable mount, `sync`'d, then `umount`+freshly-`mount_nfs`'d (to defeat the
  client page cache) and `cmp`'d against its source: **byte-for-byte identical**,
  exit 0. The post-remount read pulled the full 1 GB back *from the server* in 16 s
  (~64 MB/s, genuine server-side READ). No timeout, no corruption at GB scale; 256 MB
  passed identically too. The multi-GB ceiling is the demo FSAL's in-RAM `[]byte`,
  not the protocol. DEC-020. (AC6's eject half is exercised by the repeated clean
  remounts; **sleep-wake + signal-shutdown stay Architect-gated**.)

## Cursor — next increment

**R5 — conformance** (Milestone A's main remaining gate; the write path R6 and
R7's sustained-transfer half AC2 are banked). ([`ROADMAP.md`](ROADMAP.md))

R0→R4 read-only is done and live (DEC-018), the write path R6 is live and
race-clean, and R7's AC2 is validated to 1 GB byte-identical (DEC-020). What
remains for Milestone A:

- **R5 — read-only conformance.** Run the read-applicable `pjdfstest` subset and a
  `pynfs` NFSv4.0 read subset against a live `galatea serve` mount; enumerate
  exclusions; stand up `make test-conformance`. Runnable headless now (mounting
  works; `pjdfstest` is a C suite executed at the mountpoint).
- **R6 — the write path: the in-memory FSAL is fully read-write, proven live.**
  create/write/append/truncate/mkdir/rm/rmdir/**rename** all work over a real
  macOS NFS mount, via `NewWritableMemoryDirectory` (per-dir lock + shared inode
  counter; rename takes the two-dir lock ordered by inode) + the live-walk
  `HandleResolver`. `go test -race ./pkg/virtual` is clean. **Still open for R6:**
  (1) Mknod/Link/Symlink (still ROFS — niche; a Finder data disk rarely needs
  them); (2) the **`pjdfstest` write subset** + `make test-conformance` (R5);
  (3) `osfs` write (mutating the real disk) — a separate, later call; the
  in-memory FSAL is the read-write proving ground.
- ✅ **R1 — substrate bet VALIDATED (the founding premise).** A `slow.txt` whose
  READ sleeps 130 s (`GALATEA_SLOW_READ=130s`), mounted via NFSv4: `cat` completed
  in **2m10s, exit 0** — the macOS client held one READ RPC open >2× the ~60 s
  NFSv3 timeout window with no stall. NFSv4 dodges the RPC-timeout class that
  killed NFSv3. DEC-019.
- ✅ **`osfs` handles — done.** `osfs` now provides path-relative file handles +
  `NewHandleResolver` + the mandatory attributes (M-006 contract), tested. `galatea
  serve <host-dir> [addr]` mounts a **real host directory** — verified live: served
  the repo's `docs/` over NFSv4, `ls` listed all 7 files and `head GOAL.md` read it
  over the mount, clean `umount`. (Caveat: path handles are bounded by NFS4_FHSIZE
  ≈128 B; deeply-nested paths would need an inode/hash scheme — future.)

**R5 is the main remaining headless gate** — it hardens the read+write paths that
already work live (R6) against an external conformance suite. After R5: osfs-write
(mutating the real disk) and the niche Mknod/Link/Symlink. **Architect-gated
deferrals** (none block Milestone A's substance): a human-eyes Finder GUI
screenshot, and R7's sleep-wake/signal lifecycle half — `ls`/`mount`/`df` verify
the mount programmatically, and AC2 endurance is already measured (DEC-020).

> **⚠ Do NOT vendor `util` wholesale** — jsonnet/protobuf/grpc/prometheus/uuid;
> vendor by symbol. (Retained; applies to any future bb-storage grab, e.g. R6.)

Loop step to resume at: **2 (Scope)** for R5 — restate its "Done when", then
investigate `references/pjdfstest` (the BSD-2 C suite) against a live `galatea
serve` mount. R0→R7-AC2 is banked and green.

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
