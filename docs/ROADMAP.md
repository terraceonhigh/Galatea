# ROADMAP — the path from here to Milestone A

The ordered increments between today and [`GOAL.md`](GOAL.md). Each is a single
loop's worth of work (see [`DEVELOPMENT-LOOP.md`](DEVELOPMENT-LOOP.md)) and
carries **one verifiable gate** — "Done when" — so completion is observed, not
asserted. The cursor (which increment is in progress) lives in
[`STATUS.md`](STATUS.md), not here; this file is the plan, that file is the
position.

Increments may be re-sliced as reality teaches us — but a slice is not "done"
until its gate is green and its decision is journaled.

---

### R0 — FSAL foundation ✅ (done 2026-05-29)

The interface, two backends, a CLI driver.
**Done when:** `pkg/virtual` + `pkg/osfs` + `cmd/galatea` build and `go test ./...`
is green. ✅

### R1 — De-risk the substrate bet ✅ (done 2026-05-29, with Galatea's own server)

Confirm NFSv4 over the macOS client does **not** hit the NFSv3 RPC-timeout class
that motivated the project (the multi-minute libmtp stall).
**Done when:** a documented measurement shows a multi-minute slow read completing
over an NFSv4 mount where the v3 path stalled. ✅ **Measured:** a READ that slept
130 s server-side (`GALATEA_SLOW_READ`) completed over a live NFSv4 mount in
2m10s, exit 0 — one RPC held open >2× the ~60 s NFSv3 timeout, no stall. DEC-019.
(Run *after* the lift rather than before, since the lifted server was ready; the
bet is now confirmed with our own engine, not just FUSE-T.)

### R2 — Lift the NFSv4 server (DEC-007)

Carve bb-rex's `nfsv4` server into Galatea: vendor `path`+`filesystem`, shim
`clock`/`random`/`eviction`/`util`, replace `auth` with a localhost stub, resolve
the type fork (DEC-005). Drive it with the in-memory FSAL.
**Done when:** the lifted server package compiles and bb-rex's in-tree server
tests pass against the in-memory FSAL; `go list` shows no bb-storage import
outside the vendored floor.

### R3 — Serve NFSv4 on a socket

Wire go-xdr's `rpcserver` (TCP record-marking), AUTH_SYS, and COMPOUND dispatch
so the server answers on a loopback port.
**Done when:** `pynfs` (or a minimal client) completes a NULL call and a basic
COMPOUND (PUTROOTFH/GETATTR) against the running server.

### R4 — First Finder mount, read-only

Apply `nfsv4_mount_darwin.go`'s recipe (`NetFSMountURLAsync`/`mount_nfs`) +
DiskArbitration. Surface the `osfs` backend as a volume.
**Done when:** AC1 holds for read-only — an `osfs` mount shows in Finder and
`ls`/`cat` work at the mountpoint; clean unmount. (First "it actually mounts"
demo.)

### R5 — Conformance  🟡 (headless half ✅ done 2026-05-29; external suites gated)

Stand up `make test-conformance`; pass the read-applicable `pjdfstest` subset and
the `pynfs` NFSv4.0 read-path subset.
**Done when:** the defined read subsets pass green; exclusions enumerated.
- **Headless half ✅** — `make test-conformance` stood up; an **in-language**
  protocol-conformance suite (`internal/nfsv4/conformance_test.go`, real
  record-marked COMPOUNDs over the wire) passes 10 tests `-race`-clean: the read
  path (GETATTR/LOOKUP/READ/ACCESS/READDIR + NOENT/STALE edges), the stateless
  write path (CREATE/REMOVE/RENAME), and the **full stateful OPEN→WRITE→CLOSE
  dance** with read-back. DEC-021.
- **External suites ⛔ gated** — `pjdfstest` is non-Darwin + needs autotools +
  root (→ Linux CI on `humboldt-runner`); `pynfs` needs a `pip install ply` the
  sandbox forbids (→ one-line Architect unblock). These are the POSIX-at-mount and
  breadth-protocol complements, deferred, not skipped. DEC-021.

### R6 — Write path

FSAL write methods + NFSv4 open-state-for-write; make `osfs` and the in-memory
FSAL read-write.
**Done when:** AC3 holds — create/write/mkdir/rename/remove/truncate work through
the mount; the `pjdfstest` write subset passes.

### R7 — Endurance & lifecycle  🟡 (AC2 ✅ done 2026-05-29; AC6 partial)

Sustained multi-GB read+write; eject/sleep-wake/signal handling.
**Done when:** AC2 and AC6 hold — the sustained-transfer test completes without
timeout and the lifecycle script passes.
- **AC2 ✅** — a 1 GB random payload written to the mount, flushed, remounted (to
  defeat the client cache), and read back from the server is **byte-for-byte
  identical** (`cmp` exit 0); no timeout, no corruption at GB scale. DEC-020.
- **AC6 🟡** — clean `umount`+remount under data load is exercised repeatedly
  (eject half); **signal handling done** (`doServe` shuts down gracefully on a
  `signal.NotifyContext` SIGINT/SIGTERM cancel — `TestServeGracefulShutdown`);
  **sleep-wake remains Architect-gated** (needs a non-headless Mac).

### R8 — Milestone A acceptance  🟡 (checklist tallied 2026-05-29; tag pending gates)

Close any remaining gaps; run the full AC1–AC7 checklist.
**Done when:** all of [`GOAL.md`](GOAL.md)'s acceptance criteria are green; tag
`v0.1`. Goal redefined.
- **Checklist tallied** in [`ACCEPTANCE.md`](ACCEPTANCE.md): AC2/AC3/AC7 ✅ met
  headless; AC1/AC5 🟡 (substance met, cosmetic/tooling half gated); AC4 ⛔
  (Linux CI); AC6 🟡 (clean-unmount + signal handling done, sleep/wake gated).
  **Tag held** until AC4 + pynfs-proper land in CI so `v0.1` means the *full*
  checklist, not the headless subset.

### R9 — GOAL B: the libfuse maneuver (the FUSE-T wedge)  🟡 spike gate MET (2026-05-30)

A drop-in `libfuse.dylib` (libfuse 2.9 / macFUSE ABI) serviced by Galatea's NFS
server via a `virtual` backend that forwards to the app's `fuse_operations` — so
unmodified FUSE software (`sshfs`, `rclone mount`, …) runs on Galatea with no kext
and no closed-source daemon. Full plan in [`GOAL-B-libfuse.md`](GOAL-B-libfuse.md).
- **Phase 0 ✅** — cgo c-shared callback mechanism de-risked in three gates.
- **Phase 1a ✅** — the `fuseFS` translation layer (`shim/libfuse`), green against a
  stub `hello` ops table (`TestFuseFSTranslation`).
- **Phase 1b ✅ — LIVE READ GATE MET.** Upstream `example/hello.c`, **unmodified**,
  compiled against `libgalateafuse.dylib`, mounted on macOS and served `ls` +
  `cat "Hello World!"` through Galatea's NFSv4 server — no kext, no FUSE-T, no
  root.
- **Phase 2 ✅ — WRITE PATH.** fuseFS is read-write (write/create-via-mknod+open/
  mkdir/rename/remove/truncate/chmod). 2a: effects verified on a host temp dir
  (`TestFuseFSWritePath`, incl. the SETATTR-size=0 truncate branch). 2b live: a
  temp-dir passthrough (`fixture/passthrough.c`) mounted read-write — create+write
  lands in the backing store, cmp-identical; mkdir/rename/unlink all clean.
- **Phase 3 ✅ — the fuse_opt ABI layer.** A real tool calls more than fuse_main:
  the `fuse_opt_*` family, `fuse_get_context`, `fuse_version`. Vendored upstream
  `fuse_opt.c` (LGPL→GPL) + `fuse_compat.c` (`fuse_version`, `fuse_get_context`
  with `private_data`). Proven with `fixture/optfs.c` — a from-source FUSE program
  using `fuse_opt_parse` (`-o root=DIR`) + `fuse_main(user_data)` +
  `fuse_get_context()->private_data`, mounted live through Galatea. The shim now
  speaks the sshfs-class call pattern.
- **Phase 4 ✅ — MARQUEE: cgofuse (rclone's FUSE engine) runs read-write on the
  shim.** cgofuse — the library `rclone mount` binds through — runs unmodified on
  Galatea: built its `memfs` example, redirected its runtime `dlopen` to our dylib
  via `CGOFUSE_LIBFUSE_PATH`, and mkdir/write/read-back/rename/rm all work live (no
  kext, no FUSE-T, no macFUSE). Required closing three libfuse-lifecycle gaps the
  path-based C fixtures didn't exercise: **init/destroy**, a **pure-C readdir
  filler** (a Go-export callback re-enters our Go runtime from the app's — Go can't
  do that), and **open/release handle-bracketing** (memfs indexes an openmap by the
  fh from opendir/open). DEC in the commit (534f37e).
- **Caveats / what's left:** per-op open/release is stateless-pragmatic, not full
  handle-threading across an NFS open session. And two co-resident Go runtimes
  (rclone is Go) owe a background signal/scheduler tax — memfs is clean, heavy
  concurrent rclone is unproven. The architecturally-clean *unqualified* famous
  marquee is a **C** tool (sshfs/ntfs-3g — one Go runtime, ours), gated only on a
  build toolchain. Then the long tail (full ops, FSKit / GOAL B's endgame).

---

**Rough envelope** (solo, part-time): R1 an afternoon; R2 1–3 wks; R3–R5 ~3–4
wks; R6 3–6 wks; R7–R8 1–2 wks. ≈ 2–3.5 months to (A). See the session estimate
that produced these for the reasoning.
