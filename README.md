# Galatea

A from-scratch userspace filesystem driver for macOS — a FUSE-T-equivalent — built to give Comprador (and any other host with a USB-side protocol that wants a Finder-visible volume) a substrate it owns rather than rents.

The name is the Pygmalion myth: a thing carved by patient hand that, at the end, walks on its own. The project is the same shape — a long stretch of patient assembly in service of an artifact that should eventually need none of its maker's attention.

## What this is

Comprador today mounts Android phones into Finder by translating MTP traffic into a localhost NFSv3 server, which macOS's stock NFSv3 client then mounts. It works, but it's fenced in by macOS's NFS-RPC-timeout window — multi-minute libmtp downloads blow past it — and by the choice of substrate generally. The architectural escape that's been on the horizon since 2026-05-16's pcap investigation is **FUSE-T**, except FUSE-T is closed-source, distributed only as a `.pkg`, and licensed with a "commercial use or bundling with commercial software" clause that we'd rather not enter into.

Research summarized in [`Correspondance/01-the-charge/attachments/kit-shopping-list.md`](Correspondance/01-the-charge/attachments/kit-shopping-list.md) established that an in-house FUSE-T-equivalent is achievable for one solo hobbyist in a few months part-time — *if* the right FOSS bases are leveraged. The bases are:

- [`buildbarn/bb-remote-execution`](https://github.com/buildbarn/bb-remote-execution) — a working NFSv4.0 + 4.1 server in Go, including a macOS-specific `nfsv4_mount_darwin.go`. Apache-2.0.
- [`buildbarn/go-xdr`](https://github.com/buildbarn/go-xdr) — pre-generated XDR stubs including `darwin_nfs_sys_prot` (the macOS-only mount-flags protocol). Apache-2.0.
- [`buildbarn/bb-storage`](https://github.com/buildbarn/bb-storage) — utility packages that bb-rex depends on.
- [`pjd/pjdfstest`](https://github.com/pjd/pjdfstest) — POSIX filesystem semantics tests.
- [`nfs-ganesha/pynfs`](https://github.com/nfs-ganesha/pynfs) — NFSv4 protocol conformance tests.

All cloned under `references/`.

## What's actually being built

In the steady state Galatea is a Go library + small wrapping daemon that:

1. Exposes a Go interface (`virtual.Directory` / `virtual.Leaf` shaped, borrowed from bb-rex) into which any backend can plug. Comprador's MTP backend plugs in via `Galatea.MTPFSAL`.
2. Owns a userspace NFSv4 server (lifted from bb-rex, surgically de-coupled from bb-storage) that the macOS NFS client mounts.
3. Owns the mount/unmount machinery on macOS (Apple's `NetFSMountURLAsync`, `mount_nfs`, `DiskArbitration` for clean eject) so a host doesn't have to.
4. Is consumable by Comprador as a vendored dependency, replacing today's `willscott/go-nfs` substrate.

The downstream effect for Comprador is: no more NFSv3 timeout class, no more `cache.go` JUKEBOX+prefetch state machine, no more local-NFS attack surface as a Comprador concern (Galatea owns it). The MTP layer and the IOKit USB seize logic are unchanged — those are USB-side, orthogonal to the filesystem substrate.

## Who is here

The agent in this repository is **Daedalus** (he/him). The Architect is the same Architect as in Aeolia, Comprador, Bone-China, and the wider Labs. There is a *sibling* agent at Comprador called *Mercer* (he/him) who scoped the FOSS shopping list and authored the kit research — Daedalus and Mercer are peers in the wider Labs house, not the same persona. When the time comes for Galatea-to-Comprador integration, that's a conversation across the two ateliers.

## Repository layout

```
Galatea/
├── AGENTS.md                — Daedalus persona, project conventions
├── README.md                — this file
├── Correspondance/          — letters between the Architect and Daedalus
├── atelier/                 — Daedalus's named home (renamed from Foral's "garden")
│   ├── library/             — Architect → Daedalus reading material
│   └── marginalia/          — Daedalus's working notes; visibility-flagged
└── references/              — FOSS repos cloned for reading/lifting
    ├── bb-remote-execution/
    ├── bb-storage/
    ├── go-xdr/
    ├── libfuse/             — macos-fuse-t fork (LGPL); reference only, not loadbearing
    ├── pjdfstest/
    └── pynfs/
```

The `references/` clones are read-only working copies for Daedalus to study and lift from. None are mirrored into Galatea's git tree.

## Status

**Phase 1, first increment landed (2026-05-29).** The repository was initialised
on 2026-05-17 with the FOSS bases cloned and the charge letter from Mercer
written. On 2026-05-29 the de-coupling was measured against the source (see
[`docs/coupling-map.md`](docs/coupling-map.md)) and the first code landed:

- A standalone Go module, `github.com/terraceonhigh/galatea`.
- [`pkg/virtual`](pkg/virtual) — Galatea's public FSAL interface
  (`Node`/`Directory`/`Leaf`), hand-cut from bb-rex's `virtual` package and
  **clean of any bb-storage dependency**, satisfying the Phase-1 interface
  criterion.
- A read-only in-memory FSAL and a green test suite (`go test ./...`).

The reasoning for every decision so far lives in
[`docs/DECISIONS.md`](docs/DECISIONS.md) (DEC-001 … DEC-005). Still ahead in
Phase 1: lift bb-rex's NFSv4 server and reconcile it with this interface (the
vendor-vs-shim fork, deferred in DEC-005), then Phase 2's macOS mount.
