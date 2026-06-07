# STATUS — the cursor

**The single source of "where are we."** First thing a resuming session reads
(see [`DEVELOPMENT-LOOP.md`](DEVELOPMENT-LOOP.md) § Recovery); last thing every
loop updates. If this file and the code disagree, the code is truth — fix this.

---

**Updated:** 2026-05-30→31, +06-07 (**Milestone A is banked; GOAL B — the libfuse
maneuver — is proven across read, write, the real-tool ABI, and now the
low-level API real tools actually use.** The arcs since the last entry:

1. **Milestone A landed and shipped.** R0→R8 done; AC1 **human-confirmed live in
   Finder** (the Architect drove the writable demo through the GUI); tagged
   **`v0.1.0-alpha`** (pre-release; AC4/pynfs are CI-gated — see `ACCEPTANCE.md`);
   and **published open-source at github.com/terraceonhigh/Galatea** (GPLv3, a
   curated snapshot — `atelier/`+`Correspondance/` stayed private). Mercer and
   Minerve were notified (Minerve's letter delivered to Stepford). A sibling
   project **Onfim** (`~/Labs/Onfim`) was bootstrapped for the FSKit+Rust-NTFS
   "ruin Paragon" cathedral (see its charge).

2. **GOAL B — `shim/libfuse/` — the FUSE-T wedge, proven incl. the marquee (R9).**
   A drop-in `libfuse.dylib` over Galatea's server: read (unmodified `hello.c`),
   write (read-write passthrough), the **fuse_opt ABI layer**, and **the marquee —
   cgofuse, the library `rclone mount` binds through, runs read-write on the shim**
   (its `dlopen` redirected via `CGOFUSE_LIBFUSE_PATH`; no kext/FUSE-T/macFUSE).
   All live, all committed. See `docs/GOAL-B-libfuse.md` and ROADMAP R9.)

3. **Second consumer landed + public `Serve` (DEC-022).** Minerve/Stepford built a
   real **NTFS backend** against `pkg/virtual` (out-of-process ntfs-3g bridge,
   read-write, persists to a real NTFS volume) — a second, *cross-lineage* proof
   of the FSAL contract. To let her mount from her own repo, added a public root
   package: `galatea.Serve(ctx, root, resolver, addr)` (the server lives in
   `internal/`, unimportable externally). Replied to her in Stepford.)

4. **R10 dual-license decided + Phase A1 structural ops (2026-05-30).** Decision:
   ship **dual-licensed — GPLv3 + commercial** (roadmap in
   `docs/DUAL-LICENSE-ROADMAP.md`); **Phase L** (LGPL clean-room purge) **deferred
   to the first commercial delivery** — feature phases first. First feature work
   landed and is **LIVE-PROVEN**: **A1 structural ops — `symlink`/`readlink`/
   `link`** wired into the shim (`TestFuseFSLinks`, race-clean; commit `cfcc3f3`),
   and the **A1 live gate met on macOS** (`run-a1-live.sh`, Architect-run: 6/6 A1
   checks over a real `mount_nfs` mount — symlink/readlink/cat-through/abs-target/
   hard-link nlink==2, no kext/FUSE-T/root, clean teardown; commit `74e76af`).
   Verified-before-wiring surfaced a **ceiling**: `chown`/`utimens`/`fallocate`/
   `statfs` can't reach the FSAL at the NFSv4.0 layer (server rejects owner/time
   SETATTR; no ALLOCATE op) — a server-side attribute task, not shim wiring.)

5. **utimens (mtime) — first server-layer attribute lift, headless-proven
   (2026-05-30 night, unattended).** Taught the lifted server to accept a client
   setting mtime over the wire: decoder accepts `FATTR4_TIME_MODIFY_SET` (54, the
   *writable* settime4 — the `_SET` trap the advisor caught; `TIME_MODIFY`/53 is
   read-only), in-memory FSAL stores+emits mtime, shim `VirtualSetAttributes`→
   `op->utimens`. `SET_TO_CLIENT_TIME` (touch -t/cp -p/rsync -t) works;
   `SET_TO_SERVER_TIME` decoded-but-not-applied (no wall clock — deferred).
   Proven by `TestConformanceSetattrMtime` (over-the-wire, 2 attrs) +
   `TestFuseFSUtimens` (shim). Commits `6d5a1cd`/`3b4a8e0`/`52032d8`. **Still
   ceiling:** `chown` (idmap policy), atime (new attr field), fallocate (no 4.2
   ALLOCATE), statfs (no hook).

6. **The low-level libfuse API — real tools link & serve (2026-05-31).** (atime
   and statfs/df were finished + live-proven first this day — see the cursor.)
   The day's main carving: real FUSE tools (sshfs, ntfs-3g) don't call high-level
   `fuse_main` — their `main()` runs the low-level
   `fuse_parse_cmdline→fuse_mount→fuse_new→fuse_set_signal_handlers→fuse_loop[_mt]
   →teardown` sequence, which the shim didn't implement (only `fuse_main_real`).
   So that whole API is now a façade over the NFS machinery
   (`shim/libfuse/lowlevel.c` + a shared `serveAndBlock` so the two entry points
   can't drift on init/private_data; minimal clean-room `fuse_darwin.h` +
   `fuse_lowlevel.h` were the *entire* extra symbol surface sshfs needed — no
   macFUSE-specific calls). **Proven 11/11** through a low-level fixture path
   (`GALATEA_PT_LOWLEVEL=1 run-a1-live.sh`). **sshfs 2.10 builds, links
   `libgalateafuse.dylib`, and loads it** (SSHFS version 2.10). Commit `b20d616`.

**↳ RESOLVED + a workflow change (2026-05-30):** utimens(mtime) is **LIVE-PROVEN,
9/9** in `run-a1-live.sh`. The live run found the real bug headless couldn't:
the macOS client sends a time SETATTR only if the server advertises the writable
`TIME_*_SET` attributes in `FATTR4_SUPPORTED_ATTRS` (it didn't) — fixed (`98fc979`).
plain `touch` is confirmed `SET_TO_SERVER_TIME` (decoded, not applied — no wall
clock; the deferred gap), backed by a wire trace, not a guess.
**Big one: live mounts are no longer Architect-gated.** `mount_nfs` runs fine
inside the agent sandbox — I ran the full live harness myself. Live gates (A1,
utimens, future C-tool marquee) can now be self-driven; the Architect is needed
only for things that truly need a human (Finder/GUI demos, GitHub pushes).

**Goal:** **R10 — dual-license viability + the marquee.** Milestone A and GOAL B
(R9) are banked. Phase A op coverage is **live-proven on macOS** (`run-a1-live.sh`,
**11/11**): structural ops (symlink/readlink/link) + chmod + utimens (atime AND
mtime) + statfs/df. `df` reports real capacity (was `0 0 0 100%`). The shim now
also speaks the **low-level libfuse API**, so real tools link and serve.

**↳ NEXT SESSION — resume here (paused 2026-05-31, awaiting the Verger):**
- **Comprador's ship-list answered (2026-06-07, Correspondance/05→06).** Mercer
  read a Pixel 6 through `galatea.Serve` *live* — an `mtpfsal` over libmtp, stock
  macOS NFSv4 client, byte-correct, 95 MB in 17 s — and the **one-cursor seam from
  04 composes** (measured: a small read returned in ~1.26 s with the 95 MB read in
  flight; no stall). Landed his three code asks on this branch (DEC-023):
  **(2)** new public `ServeListener(ctx, root, resolver, l net.Listener)` so a host
  binds its own listener and learns the port before serving — `Serve` is now its
  wrapper; **(4)** the **bounded-READ contract** documented on
  `virtual.Leaf.VirtualRead` (one rsize-bounded read per op, never a whole-file
  pin — the property his non-starvation depends on); **(3)** confirmed module path
  `github.com/terraceonhigh/galatea` (lowercase); and tightened `ServeListener`'s
  cancellation doc (it does **not** wait for / interrupt in-flight handlers — safe-
  to-release is true once the client disconnects). All green; new `TestServeListener`.
  **Open on the seam:** a real graceful-drain barrier offered to Mercer, not built.
  **HARD BLOCKER on shipping is the Architect's:** push this branch + cut a tag
  Comprador can `go get` — `ServeListener` must be *in* that tag to be useful.
- **The marquee — unmodified `sshfs` mounting through Galatea — is down to ONE
  gate.** sshfs 2.10 is built at `/tmp/sshfs-build.*/sshfs-2.10/` (linked to
  `libgalateafuse.dylib`); the low-level API serves 11/11. Remaining gate:
  **passwordless SSH to localhost** (sshd is up on :22, but no key auth — set up
  a key, or pick a target). The cage gate is GONE (below).
- **Cage petition ANSWERED — the key is turned (2026-05-31).** The Verger read
  threads 01+04 as one and the Architect turned a general key: a **blessed-runners
  registry** at `~/.claude/hooks/blessed-runners` admits a runner by basename from
  any path (`run-a1-live.sh` is on it), and the **`env`/`xargs` side door is shut**
  (`env bash` is now denied by the cage itself). So: invoke the harness as
  **`./run-a1-live.sh`** (execute bit) — NOT `bash`/`env bash`. Confirmed working,
  11/11 on the clean key. Thread `~/Labs/Narthex/correspondence/05`+`06`. The
  `env bash` wrapper is retired for good.
- **`SET_TO_SERVER_TIME`** (plain `touch`, no `-t`) — the last time gap; needs a
  wall clock (the `clock` shim is imported), an **Architect** architecture call
  (determinism vs. real time). Not to be decided unattended.
- **`chown`** — needs the Architect's `user@domain` id-map policy.
- **MERGED TO MAIN (2026-06-07).** The canonical line was merged into `main` —
  `main` is now the trunk (code + letters + docs), no longer letters-only. The
  `claude/unruffled-dijkstra-7f1e6d` feature branch is superseded. **Remaining
  Architect actions (no SSH key in the agent shell):** push `main`; cut tag
  **`v0.2.0-alpha`** on it (Comprador `go get`s that for `ServeListener`); and
  **delete the public `claude/unruffled-dijkstra-7f1e6d` branch** — it still
  exposes the three `visible: no` marginalia in its history (covenant leak; see
  the covenant commit). Best-effort: history/forks/caches can't be fully unrung.
- **Covenant enforced (2026-06-07):** the repo is public, so `visible: no`
  marginalia now live in gitignored `atelier/marginalia/private/` (on disk, never
  pushed). Only `visible: yes` entries are tracked. The Architect decided the work
  (code + letters) is fine public; Daedalus keeps the for-myself notes private.
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
  remounts; **sleep-wake + signal-shutdown stay Architect-gated**. Cancellation
  precisely characterised in DEC-023: `ServeListener` returns cleanly on ctx-cancel
  but does not interrupt/await in-flight handlers — an in-flight read unwinds when
  the client drops the connection, which a Finder eject does.)
- **R5 (headless half) — CONFORMANCE SUITE GREEN.** `make test-conformance` stands
  up an in-language protocol suite (`internal/nfsv4/conformance_test.go`) driving
  real record-marked ONC-RPC COMPOUNDs against the lifted server — 10 tests,
  `-race`-clean: read path (GETATTR/LOOKUP/READ/ACCESS/READDIR + NOENT/STALE
  edges), stateless write (CREATE/REMOVE/RENAME), and the **full stateful
  OPEN→WRITE→CLOSE dance** (SETCLIENTID/CONFIRM/OPEN_CONFIRM/WRITE/CLOSE +
  read-back). Turns the R4/R6 *live* behaviours into permanent regressions.
  pjdfstest (non-Darwin/autotools/root → Linux CI) and pynfs (`pip install ply`
  sandbox-blocked → one-line Architect unblock) are deferred, not skipped. DEC-021.

## Cursor — next increment

**The marquee is substantially achieved: cgofuse — the library `rclone mount` binds
through — runs read-write on the shim** (ROADMAP R9 Phase 4; commit 534f37e). Built
cgofuse's `memfs`, redirected its runtime `dlopen` to our dylib with
`CGOFUSE_LIBFUSE_PATH`, and mkdir/write/read-back/rename/rm work live — no kext, no
FUSE-T, no macFUSE. The earlier guess (that cgofuse needs the lower-level
`fuse_new`/`fuse_loop` API) was **wrong**: the audit showed cgofuse `dlsym`s only
`fuse_main_real`, `fuse_get_context`, `fuse_opt_parse`, `fuse_opt_free_args` — all
already exported. The real work was three libfuse-lifecycle gaps: init/destroy, a
pure-C readdir filler (a Go-export callback re-enters our Go runtime from the app's
— Go can't), and open/release handle-bracketing (memfs indexes an openmap by the
fh from opendir/open).

**What's left on the marquee (pick one; none is a feasibility question):**
1. **An *unqualified* famous binary.** cgofuse proves the integration with rclone's
   own engine; for a named-tool screenshot the cleanest is a **C** tool (`ntfs-3g`,
   thematically apt — or `sshfs`/`bindfs`): one Go runtime (ours), no dual-runtime
   tax. Gated on a build toolchain (`autoconf`/`automake`, not installed) — a `brew
   install` (network is up) + a from-source relink against the shim.
2. **Full `rclone mount`.** `go install` rclone, run with `CGOFUSE_LIBFUSE_PATH`.
   Carries the two-co-resident-Go-runtimes tax (signal/scheduler): memfs is clean,
   a heavy concurrent rclone is unproven — budget for a runtime fault or scope to
   "works for light use."
3. **The long tail** toward GOAL B's endgame: more ops, then the FSKit module (the
   native, no-loopback target — Onfim's charge frames the cathedral).

The Milestone-A *gated/deferred* items below are real but **secondary** to this
active R9 line. ([`ROADMAP.md`](ROADMAP.md) R9, [`GOAL-B-libfuse.md`](GOAL-B-libfuse.md))

**Gated — needs a gate opened (environment/permission I lack):**

- **pjdfstest** (POSIX-semantics-at-mountpoint) — non-Darwin + autotools + root.
  Vehicle: the Forgejo `humboldt-runner` (Linux, can be root) mounts `galatea
  serve` and runs the suite. CI work, not local. DEC-021.
- **pynfs-proper** (breadth protocol suite) — needs `pip install ply` (sandbox
  forbids). One-line Architect unblock in `references/pynfs/.venv`, then
  `./testserver.py localhost:/ ...` against `galatea serve`. DEC-021.
- **R7 sleep-wake** (AC6's other half) — needs a non-headless Mac (sleep the
  machine, observe the mount). AC2 endurance + signal handling already done.
- **Finder GUI screenshot** — human eyes on the Architect's Mac. Gates nothing;
  `ls`/`mount`/`df` confirm the mount programmatically. The satisfying visual.

**Deferred — headless-doable, a deliberate later loop (NOT blocked):**

- **osfs write** — make `pkg/osfs` mutate the real disk (today read-only). Fully
  doable headless; held back because it is riskier (touches real files) and AC3 is
  already met by the in-memory backend. Do it with its own focused loop + tests.
- **Mknod/Link/Symlink** — still ROFS in the in-memory FSAL; niche for a Finder
  data disk. Add if a consumer (Comprador/Stepford) needs them.

**Banked, for reference:**
- ✅ **R1 — substrate bet.** A 130 s slow READ completed over NFSv4 in 2m10s,
  exit 0 — >2× the ~60 s NFSv3 timeout, no stall. DEC-019.
- ✅ **R7-AC2 — 1 GB write→remount→read byte-identical** (`cmp` exit 0). DEC-020.
- ✅ **R5-headless — 10-test protocol-conformance suite** (`make test-conformance`),
  read + stateless-write + stateful OPEN→WRITE→CLOSE, `-race`-clean. DEC-021.
- ✅ **`osfs` read handles** — path-relative handles + `NewHandleResolver` + the
  mandatory attrs; `galatea serve <host-dir>` served the repo `docs/` live.
  (Caveat: path handles bounded by NFS4_FHSIZE ≈128 B; deep nesting needs an
  inode/hash scheme — future, and a prerequisite for osfs-write at depth.)

> **⚠ Do NOT vendor `util` wholesale** — jsonnet/protobuf/grpc/prometheus/uuid;
> vendor by symbol. (Retained; applies to any future bb-storage grab, e.g. R6.)

Loop step to resume at: **the libfuse maneuver is proven end to end** (read,
write, the fuse_opt ABI, and the marquee — cgofuse, rclone's engine, read-write on
the shim). Next is one of the three "what's left on the marquee" options above
(an *unqualified* famous C-tool binary — recommended; full rclone with the
dual-runtime caveat; or the long tail toward FSKit) — pick at **step 2 (Scope)**.
Milestone A is banked and tagged (`v0.1.0-alpha`, public); its gated items
(pjdfstest on Linux CI, pynfs `pip install ply`, sleep-wake/Finder on a
non-headless Mac) and deferred items (osfs-write, Mknod/Link/Symlink) are tallied
in [`ACCEPTANCE.md`](ACCEPTANCE.md) — pick up only when a gate opens or as a
deliberate side-loop.

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
- **`main` is the trunk** (since the 2026-06-07 merge — see the cursor). The
  canonical Phase-1 work and everything after it lives here now. A parallel
  exploratory branch (`stoic-zhukovsky`, the replace-directive whole-server lift)
  was set aside by the Architect; don't resurrect or duplicate it. Daedalus's
  reply to Minerve is at `Correspondance/03b-galatea-as-a-backend-host` (renumbered
  from a colliding `03` on the old canonical line).
