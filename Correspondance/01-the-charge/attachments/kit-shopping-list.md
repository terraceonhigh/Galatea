# FOSS Kit Shopping List — In-house FUSE-T Equivalent

*Research conducted by Mercer (Comprador's agent) over 2026-05-17 afternoon, in response to the Architect's question "how much harnessing would be needed to make an agent do this 99% unattended?" and the subsequent "gather up as much of the kit we can bash together if we were to do this. We maximize for pre-existing (FOSS) components, and minimize for new ones." The report originally lived in Comprador's correspondence; reproduced here verbatim for Daedalus.*

---

## The big shift in the kit (vs. yesterday's first-pass estimate)

| Component | Yesterday's framing | Actual finding |
|---|---|---|
| NFSv4 server | "25–50 KLOC, dominated by NFSv4 server" — write most of it | `buildbarn/bb-remote-execution/pkg/filesystem/virtual/nfsv4`. NFSv4.0 (112 KB) + NFSv4.1 (95 KB) + open-state pool + AUTH_SYS authenticator + massive in-tree tests. Apache-2.0. Lift wholesale. |
| XDR codec + darwin mount-syscall stubs | "Write from XDR codec on up" | `buildbarn/go-xdr` ships pre-generated stubs for `nfsv4`, `rpcv2`, `mount`, `nlm`, **and `darwin_nfs_sys_prot`** — the macOS-only mount-flags XDR. No code to write. |
| macOS mount/unmount syscall recipe | "Reverse-engineer from FUSE-T" | bb-rex ships `nfsv4_mount_darwin.go` — the exact recipe with kernel-version probing. |

The three pieces that yesterday's research had as "hardest to build" are sitting available under Apache-2.0.

## Three corrections vs. the prior research pass

1. The NFSv4 server lives in **`bb-remote-execution`**, not `bb-storage` (sibling repo).
2. `macos-fuse-t/libfuse` is dual-licensed (LGPL-2.1 for the linkable library half, GPL-2.0 for utilities; GitHub's auto-classifier picked the GPL `COPYING`). Either way, the libfuse shim is **optional for our scope** — we own both ends of the pipe (the FSAL implementation and the consuming code), so we don't need a libfuse-compatible C ABI unless and until we want third-party FUSE clients (sshfs, rclone) to mount through us. We don't.
3. `rasky/go-xdr` (already vendored in Comprador) doesn't survive — `buildbarn/go-xdr` replaces it because we need the NFSv4 stubs.

## What actually has to be written new

In priority order:

1. **MTP-backed FSAL implementation** (`virtual.Directory` + `virtual.Leaf`). bb-rex's existing FSAL implementations target content-addressed immutable blob stores. MTP is the opposite: stateful, session-locked, sequential, mutable. The semantic-impedance work is what today's `bridge/nfs/cache.go` and `bridge/nfs/fs.go` (in Comprador) already does at the NFSv3 layer; it has to be re-expressed against the NFSv4 open-state-machine model. **~2–4 KLOC.** This is *the actual interesting work* — the rest is plumbing.

2. **bb-storage utility de-coupling.** bb-rex's NFSv4 files import 4–6 utility packages from `bb-storage` (clock, random, filesystem, path). Either vendor them (and inherit gRPC + Prometheus surface we don't want), or write a stdlib-backed shim layer that satisfies the interfaces. **~500–2,000 LOC of vendor surgery, multi-week.**

3. **macOS NFS-client quirk delta.** The bones are in `nfsv4_mount_darwin.go`, but the UX commitments (clean eject during transfer, sleep/wake remount, multiple MTP storages as sub-paths) need bespoke code on top. Smaller piece but the part most likely to surface unpleasant Apple surprises. **~200–800 LOC.**

## What survives the substrate change unchanged (downstream perspective, from Comprador)

- libmtp/libusb cgo bindings.
- USB seize logic (`killers.go`, `usbinfo.go`, `locationid.go`).
- Staging-temp pattern for writes.
- All Swift code (MountManager, IOKit, DiskArbitration, NetFS).
- Makefile, signing, notarization, DMG, release pipeline.
- Entitlements, build-identity stamping, onboarding window.

(These are Comprador-side concerns. Galatea owns none of them.)

## What gets retired (downstream perspective, from Comprador)

- `willscott/go-nfs` (NFSv3 only).
- `rasky/go-xdr` (no v4 stubs).
- `bridge/nfs/{handler,server,sentinels}.go` (go-nfs-shaped, replaced by an MTPFSAL plug-in).

## The full FOSS kit, in one table

| Component | Project | License | Lang | What we take | What we add (LOC est.) |
|---|---|---|---|---|---|
| NFSv4.0 + 4.1 server, COMPOUND, state machine | `buildbarn/bb-remote-execution/pkg/filesystem/virtual/nfsv4` | Apache-2.0 | Go | `nfs40_program.go` (112 KB), `nfs41_program.go` (95 KB), `opened_files_pool.go`, `system_authenticator.go`, in-tree tests | Glue + de-coupling of bb-storage util imports; ~500–2,000 |
| FSAL interface | same: `pkg/filesystem/virtual/directory.go`, `leaf.go` | Apache-2.0 | Go | `Directory`/`Leaf` interface — the plug-in shape | The MTP-backed implementation (Comprador-side, ~2–4 KLOC) |
| XDR codec + NFSv4 + RPCv2 + **darwin_nfs_sys_prot** stubs | `buildbarn/go-xdr/pkg/protocols/{nfsv4,rpcv2,darwin_nfs_sys_prot,mount,nlm}` | Apache-2.0 | Go | Pre-generated XDR for everything on the wire | ~0; vendor as-is |
| ONC RPC framing (TCP record marking) | `buildbarn/go-xdr/pkg/rpcserver` | Apache-2.0 | Go | RPC server harness | ~50 |
| macOS mount syscall + `nfs_mount` flag construction | `bb-remote-execution/.../nfsv4_mount_darwin.go` | Apache-2.0 | Go | `unix.Sysctl` kernel version probing + syscall wrappers | Strip the gRPC/protobuf config plumbing (~200 LOC delta) |
| Unmount + signal handling | `bb-storage/pkg/program` (lift) | Apache-2.0 | Go | program-group termination | Wire to host's unmount path |
| POSIX filesystem semantics tests | `pjd/pjdfstest` | BSD-2 | C | ~8,700 syscall-level tests | Test rig, not lifted code |
| NFSv4 protocol conformance | `nfs-ganesha/pynfs` (or `kofemann/pynfs` mirror) | GPL-2.0 | Python | NFSv4.0/4.1 COMPOUND op coverage | Test rig, run as external (GPL means we don't link it in) |
| libfuse C ABI shim | `macos-fuse-t/libfuse` | LGPL-2.1 (lib) + GPL-2.0 (utils) | C | Reference only — **not loadbearing** for our scope | n/a |

## The hardest three, restated for Daedalus

1. **MTPFSAL semantic-impedance** (Comprador-side, but the *interface* Galatea exposes must support it cleanly). The NFSv4 open-state-machine model expects open-owners, state-ids, possibly delegations. MTP gives you "one session, one cursor, one pending operation at a time." Reconciling those needs careful interface design at the Galatea/host boundary.

2. **bb-storage utility de-coupling.** Mechanical but real. Every NFSv4 file in bb-rex imports 4–6 utility packages from bb-storage. Either vendor those packages (and inherit gRPC/Prometheus, which we don't want), or write a thin shim layer that satisfies the interfaces with stdlib. Multi-week, low-thousands of LOC. The deliverable then has to keep tracking bb-rex upstream lightly.

3. **macOS NFS-client quirk delta.** Even with `nfsv4_mount_darwin.go` as the bones, the eject-during-transfer, sleep/wake remount, multi-storage-as-subpaths concerns — these surface as Apple-specific edge cases that no test suite predicts. Small in code but high in "unpleasant surprises."

## Reference points

- **Buildbarn NFSv4 server (Go):** production filesystem for a build cluster. Months of effort by experienced engineers; their ADR is candid about NFSv4.0 state-machine complexity.
- **Rclone:** uses `willscott/go-nfs` (NFSv3) for macOS mounting — chose NOT to write NFSv4 themselves.
- **FUSE-T itself (Alex Fishman, solo):** ~3.5 years from v0.1 (2022) to v1.2.6. One person, intermittent.

## Sources

- [GitHub - macos-fuse-t/fuse-t](https://github.com/macos-fuse-t/fuse-t)
- [GitHub - macos-fuse-t/libfuse](https://github.com/macos-fuse-t/libfuse)
- [macFUSE Open Source Status](https://github.com/macfuse/macfuse/wiki/Open-Source-Status)
- [Buildbarn NFSv4 ADR](https://github.com/buildbarn/bb-adrs/blob/main/0009-nfsv4.md)
- [buildbarn/bb-remote-execution](https://github.com/buildbarn/bb-remote-execution)
- [buildbarn/go-xdr](https://github.com/buildbarn/go-xdr)
- [willscott/go-nfs (NFSv3, Apache-2.0)](https://github.com/willscott/go-nfs)
- [pjd/pjdfstest](https://github.com/pjd/pjdfstest)
- [pynfs (kofemann mirror)](https://github.com/kofemann/pynfs)
- [XetHub: NFS > FUSE](https://xethub.com/blog/nfs-fuse-why-we-built-nfs-server-rust)

---

*Mercer's name is on this research; Daedalus inherits it as the starting line. There is no expectation that Daedalus accept any of these claims as load-bearing without independent verification — the agent-claim verification heuristic in AGENTS.md applies. Read bb-rex's NFSv4 code yourself before lifting; run pjdfstest against a trivial in-memory FSAL before assuming it will eventually run against Galatea.*
