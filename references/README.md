# references/

Third-party FOSS repositories cloned for Daedalus to read and selectively lift from. None are mirrored into Galatea's git tree — `.gitignore` excludes this directory.

Cloned 2026-05-17 with `--depth=1` (history not preserved; refetch if you need it).

| Repo | License | Role | Loadbearing? |
|---|---|---|---|
| `bb-remote-execution/` | Apache-2.0 | NFSv4.0 + 4.1 server in Go. `pkg/filesystem/virtual/nfsv4/` is the lift target. Contains `nfsv4_mount_darwin.go`. | **Yes** — primary lift |
| `bb-storage/` | Apache-2.0 | Utility packages that `bb-remote-execution` imports (`clock`, `random`, `filesystem`, `filesystem/path`). Either vendor selectively or write shims. | **Yes** — surgical lift |
| `go-xdr/` | Apache-2.0 | Pre-generated XDR codec for NFSv4, RPCv2, and crucially `darwin_nfs_sys_prot` (the macOS-only mount-flags XDR). Use directly as a Go dependency. | **Yes** — vendor as-is |
| `libfuse/` | LGPL-2.1 (lib) + GPL-2.0 (utils) | `macos-fuse-t`'s fork of libfuse. **Not needed for Comprador scope** (we own both ends of the FSAL/consumer pipe). Kept here as reference for the libfuse C ABI if a downstream ever asks for it. | No — reference only |
| `pjdfstest/` | BSD-2 | ~8,700 POSIX syscall-level filesystem tests (`chmod`, `mkdir`, `rename`, `open`, etc.). Run against the eventually-mounted Galatea volume. | **Yes** — test harness |
| `pynfs/` | GPL-2.0 | NFSv4.0 and NFSv4.1 protocol-conformance tests (COMPOUND, state, lock, session). Use for development; GPL-2.0 means we can't link it in, only run as external. | **Yes** — test harness |

## What "lifting" means

For the Apache-2.0 sources (bb-remote-execution, bb-storage, go-xdr), we expect to:

1. **Vendor selectively** into Galatea's own module tree, preserving copyright headers, with our own `vendor/` Go module imports.
2. **Patch as needed** to de-couple from each repo's broader surface (bb-storage's gRPC/Prometheus plumbing, bb-rex's protobuf configuration system).
3. **Track upstream** lightly — re-pull and re-merge when an upstream change is interesting; not a hard tracking obligation.

`pjdfstest` and `pynfs` are runtime test harnesses, not code we lift. They get invoked by Galatea's CI / `make test-conformance` targets.

`libfuse` is reference-only. If Galatea ever needs to expose a libfuse-compatible C ABI (so external FUSE clients — sshfs, rclone — can mount through Galatea instead of FUSE-T), it would be a separate downstream effort built on top of Galatea's Go-native interface.

## Refetching with full history

If you need full git history for any of these (to bisect, to read PR discussions, to find when a function was introduced):

```
cd references/bb-remote-execution
git fetch --unshallow
```

## .gitignore

The top-level `.gitignore` excludes `references/`. These clones do not enter Galatea's git tree.
