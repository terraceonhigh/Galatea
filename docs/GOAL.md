# GOAL — Milestone A

This file defines Galatea's **current ultimate goal**. It is the destination
every increment is measured against. It is deliberately narrow: not the whole
FUSE-T/macFUSE ambition, but the one bounded thing that, once true, makes Galatea
a real product for its actual purpose.

When the acceptance criteria below all pass, Milestone A is done, tagged `v0.1`,
and the goal is redefined (likely toward "(B) — a modest FUSE-T equivalent for
third-party libfuse programs"). Until then, **this is the target.**

---

## The shape of (A), in one sentence

> A read-write, POSIX-reasonable, **Finder-visible filesystem of our own** —
> mounted on macOS with no kernel extension and no closed-source or
> commercially-licensed dependency — that a host program drives by implementing a
> Go backend and calling `galatea.Mount(...)`.

It is the hand-cut equivalent of what FUSE-T's `.pkg` gives, for filesystems
*we* build. It is **not** a drop-in for other people's FUSE programs (that needs
the libfuse C ABI — a later goal), and it is **not** the MTP backend itself
(that is Comprador's work, Phase 4).

## The consumer-facing contract

A host obtains a mount in three steps:

```go
// 1. Implement a read-write backend satisfying Galatea's FSAL.
//    (osfs is the reference backend; Comprador's MTPFSAL is the eventual
//    real consumer.)
var root virtual.Directory = mybackend.Root(...)

// 2. Mount it.
mount, err := galatea.Mount(root, galatea.Options{
    VolumeName: "MyVolume",
    MountPoint: "/Volumes/MyVolume", // or auto
})

// 3. Use it through Finder / any POSIX program, then release it.
defer mount.Unmount()
```

`galatea.Mount` owns everything between the backend and the OS: the userspace
NFSv4 server, the RPC framing, the macOS mount syscalls, and clean
unmount/eject. The host writes a backend and nothing else.

## Capabilities (A) must have

- **Read:** lookup, readdir, getattr, read (streaming, arbitrary size).
- **Write:** create, write, mkdir, rename, remove/unlink/rmdir, setattr,
  truncate.
- **Lifecycle:** mount, clean unmount, eject-during-idle, SIGINT/SIGTERM →
  graceful unmount, survive (or gracefully remount after) sleep/wake.
- **Endurance:** sustained multi-gigabyte read *and* write without hitting the
  macOS NFS-client RPC-timeout class — the failure that motivated the project.

## Acceptance criteria (all must pass)

Each is an empirical gate, not a claim. The roadmap drives toward them in order.

| # | Criterion | How it's verified |
|---|---|---|
| **AC1** | An `osfs`-backed mount appears as a Finder volume showing the real tree | mount, eyeball in Finder, `ls`/`cat` at the mountpoint |
| **AC2** | A multi-GB file reads out through the mount, byte-identical, no timeout | copy out via Finder + `cmp`; watch for RPC stall |
| **AC3** | create / write / mkdir / rename / remove / truncate all work through the mount | scripted POSIX ops at the mountpoint, reflected in the backend |
| **AC4** | `pjdfstest`'s applicable subset passes | `make test-conformance` (POSIX); exclusions enumerated in `test/` |
| **AC5** | `pynfs` NFSv4.0 COMPOUND-op conformance subset passes (4.1 best-effort) | `make test-conformance` (protocol) |
| **AC6** | Clean unmount, eject-while-idle, signal handling, sleep/wake | lifecycle test script + manual sleep/wake |
| **AC7** | No closed-source dep, no kext, no commercial-license exposure | `go list` shows no bb-storage outside the vendored floor; license audit |

## Non-goals of (A) — explicitly out of scope

- The **libfuse C ABI** (sshfs/rclone drop-in). That is goal (B).
- The **MTP backend**. Comprador's job, Phase 4; (A) ships with `osfs` + the
  in-memory FSAL as its backends.
- **Write-performance tuning** beyond "does not time out."
- **macFUSE feature parity** — architecturally unreachable over NFS (device
  nodes, certain ioctls, multi-user ownership/ACLs). Not attempted.
- **Non-macOS platforms.**

## Definition of done

All seven ACs green; the conformance subsets defined and passing via
`make test-conformance`; tagged `v0.1` ("Milestone A"). At that point this file
is superseded by the next goal.
