# GOAL B — the libfuse maneuver (the FUSE-T wedge)

Status: **the maneuver is proven through a real third-party FUSE library
(2026-05-30).** Phases 0–2 (cgo mechanism; translation + live read mount of
unmodified `hello.c`; live read-write passthrough), Phase 3 (the `fuse_opt` ABI
layer), and Phase 4 — **the marquee: cgofuse, the engine `rclone mount` binds
through, runs read-write on the shim** (its runtime `dlopen` redirected via
`CGOFUSE_LIBFUSE_PATH`; mkdir/write/read/rename/rm live). No kext, no FUSE-T, no
macFUSE. Code is `shim/libfuse/`. What remains is an *unqualified* famous-named
binary (a C tool — one Go runtime; or full rclone with the two-Go-runtime caveat)
and the long tail (full ops, then GOAL B's FSKit endgame) — breadth, not
feasibility. This is the plan for Galatea's second goal (see
[`GOAL.md`](GOAL.md) — "the libfuse C ABI / sshfs-rclone drop-in"). Milestone A
(read-write NFSv4 mount on macOS, unprivileged) is the substrate this builds on
and is functionally complete (see [`ACCEPTANCE.md`](ACCEPTANCE.md)).

## Why this is the move

FUSE-T's moat is not "userspace NFS" — we share that insight. Its moat is the
**libfuse C ABI**: every macFUSE-era tool (`sshfs`, `rclone mount`, `s3fs`,
Cryptomator, `restic`/`borg mount`) links libfuse and works on FUSE-T with a
relink. That ecosystem *is* its userbase. Its underbelly is that it's
closed-source, carries a bespoke commercial-use clause, and has a bus factor of
one. We win the FOSS + audit-conscious segment **only if their existing software
can run on us** — i.e. only if we provide a drop-in `libfuse.dylib`. Without it we
are a superb Go-native substrate (Comprador, Stepford) but not a FUSE-T
replacement. With it, the pitch is: *"your existing FUSE software, no kext, no
closed-source daemon, no license to read twice, and you can fix it yourself."*

Note for honesty in our own messaging: the slow/large-transfer proofs (R1, R7)
ruined **NFSv3's** day, not FUSE-T's — FUSE-T is also NFS-based and clears the
same bar. The wedge against FUSE-T is **open + license-clean + auditable +
Go-embeddable**, delivered through ABI compatibility. Do not market the transfer
story as a FUSE-T differentiator.

## The shape

The shim is **just another Galatea backend plus a C front door.** We already have
`NFS server ← virtual.Directory/Leaf ← backend`. This adds:

1. A drop-in **`libfuse.dylib`** exposing libfuse 2.9's exact C ABI
   (`fuse_main`, `struct fuse_operations`, helpers). An app links `-lfuse`, unaware.
2. A **`virtual` backend that forwards to the app's `fuse_operations`**: when an
   NFS READ arrives, our server calls the app's `read(path,…)` and replies with
   the result.

Data flow: `app → fuse_main(ops) → dylib starts our NFS server in-process & mounts
it → kernel NFS client → our NFS server → calls the app's C ops → replies over
NFS`. This is exactly FUSE-T's trick, mapped onto our existing server — the server
is done; we give it a new face and a new backend.

## The four work-pieces

**1. The C ABI surface.**
- Target **libfuse 2.9** — specifically **macFUSE's flavor** (2.9 + macOS
  extensions: `setvolname`, `exchange`, `getxtimes`, …). Matching the de-facto Mac
  ABI is what makes real Mac apps work, not just `sshfs`.
- Reuse the **LGPL upstream headers + high-level path layer** (libfuse's `fuse.c`
  already turns path-ops into the inode model and back) — verbatim, see
  `references/libfuse/` (libfuse 2.9, `COPYING.LIB`). Do **not** lift FUSE-T's
  proprietary NFS bridge (it isn't in that tree anyway; the bridge is ours).
- Symbols apps actually call: `fuse_main`/`fuse_main_real`, `fuse_new`/`fuse_mount`/
  `fuse_loop`/`fuse_unmount`/`fuse_destroy`, `fuse_get_context`, option parsing
  (`fuse_opt_parse`, `fuse_parse_cmdline`), `fuse_version`.

**2. The cgo boundary (the genuinely hard part).**
- Build with `go build -buildmode=c-shared -o libfuse.dylib`. **This is the one
  component where we accept CGO** — the core stays pure-Go; the shim is the edge.
- Hard direction: *Go calling the app's C function pointers* (`fuse_operations`)
  with byte-exact `struct` layout and correct threading (honor `-s`
  single-threaded; app callbacks may not be reentrant).
- Synergy: `fuse_get_context()` wants caller uid/gid/pid — we already have uid/gid
  from the **AUTH_SYS credential** on every NFS call. Free.

**3. The operation-model translation.**
- Path ↔ node ↔ NFS-handle (libfuse high-level gives path↔inode; we bridge
  inode↔FSAL↔filehandle — our `HandleResolver` is the seed).
- errno ↔ NFS status (we already speak `NFS4ERR_*`).
- Open file-handles, readdir cookies, `stat` ↔ FATTR4 (we already do FATTR4).
- **Ceiling (shared with FUSE-T):** `ioctl`, `poll`, device nodes, some locking
  don't map to NFS. Document unsupported; not a deficiency unique to us.

**4. Lifecycle.** `fuse_main` blocks and runs the loop; mount/unmount, option
passthrough, signal handling (we already have graceful signal shutdown).

## Licensing

`references/libfuse` is libfuse 2.9 under **LGPL 2.1**. LGPL → GPLv3 is compatible,
so we may incorporate the upstream **headers + high-level path layer** into GPLv3
Galatea. The NFS-bridge backend is **novel work we author**. No trap, provided we
reuse *upstream* libfuse, not FUSE-T's closed bridge.

## R9 — the spike (≈1–2 focused weeks)

**Goal:** prove the ABI holds end to end with the smallest real surface.

- Implement `fuse_main` + a **~10-op high-level subset**, each mapped to a
  `virtual` method:
  | FUSE op | → virtual / NFS |
  |---|---|
  | `getattr` | `VirtualGetAttributes` |
  | `readdir` | `VirtualReadDir` |
  | `open` / `release` | `VirtualOpenSelf` / `VirtualClose` |
  | `read` | `VirtualRead` |
  | `write` | `VirtualWrite` |
  | `create` | `VirtualOpenChild` (create) |
  | `mkdir` | `VirtualMkdir` |
  | `unlink` / `rmdir` | `VirtualRemove` |
  | `rename` | `VirtualRename` |
  | `truncate` | `VirtualSetAttributes` (size) |
- A `fuseOpsBackend` implementing `virtual.Directory`/`Leaf` over the C ops, plus a
  path↔node map (reuse the in-memory/handle-resolver pattern, or libfuse's path
  layer).
- **Gate (Done when):** libfuse's own `references/libfuse/example/passthrough`
  filesystem, **unmodified**, mounts through Galatea's `libfuse.dylib` and
  round-trips a file (write, read-back identical, `ls`, mkdir, rm) over the mount.
  Journal a DEC with the receipt.

## The campaign (after the spike)

- **Marquee (the launch):** unmodified **`sshfs`** (or `rclone mount`) mounting
  read-write through `libfuse.dylib`. That screenshot is what takes FUSE-T's users.
- **Long tail (incremental, months):** full op coverage, the low-level API
  (`fuse_lowlevel_ops`), xattrs, the macFUSE macOS extensions, then FUSE 3.x as a
  separate ABI. Driven by which real apps we want to claim.

## Risks to watch
- **cgo callback threading/correctness** — biggest unknown; the spike de-risks it.
- **Struct-layout exactness** — get `fuse_operations` byte-perfect or it crashes.
- **FUSE 2 vs 3** — 2.9 is the widest net and the right first target; 3.x later.
- **macFUSE Mac extensions** — matching *macFUSE's* surface (not generic libfuse)
  is what makes Mac-native apps work, not just `sshfs`.

## The proof that ends the argument
An unmodified, off-the-shelf FUSE filesystem mounting read-write on Galatea — no
kext, no closed-source daemon. The day `passthrough` works, the maneuver is real.
The day `sshfs` works, it's a product.
