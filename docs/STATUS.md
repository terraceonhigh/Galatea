# STATUS — the cursor

**The single source of "where are we."** First thing a resuming session reads
(see [`DEVELOPMENT-LOOP.md`](DEVELOPMENT-LOOP.md) § Recovery); last thing every
loop updates. If this file and the code disagree, the code is truth — fix this.

---

**Updated:** 2026-05-29 (autonomous run: R1 gated; R2a + serving-foundation +
**R2b + R2c** done; (A) confirmed reachable; cursor at R2d — the server lift)
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

## Cursor — next increment

**R2d — lift bb-rex's NFSv4 server into Galatea (the heart of R2).**
([`ROADMAP.md`](ROADMAP.md) R2)

> **Done when:** the lifted server package compiles in-tree against `pkg/virtual`
> + the vendored `internal/bb` / `internal/xdr`, with `go build/vet/test/fmt
> ./...` green and `go list` showing no `buildbarn/*` import. (bb-rex's gomock
> test mountain is out of scope — DEC-011; a Galatea smoke COMPOUND test is the
> behavioural gate, folded into R3.)

R2a/R2b/R2c are done: the wire codec (`internal/xdr`), the leaf types
(`internal/bb`), and the interface (`pkg/virtual`) are all in place and on the
*same* types the server speaks — so the lift is import-rewrite + shim, not
redesign. Remaining work for R2d, from the coupling map:

- **Copy** `nfs40_program.go`, `nfs41_program.go`, `opened_files_pool.go`,
  `minor_version_fallback_program.go`, `metrics_program.go` from bb-rex's
  `pkg/filesystem/virtual/nfsv4/` into Galatea (likely `internal/nfsv4/` or
  `pkg/nfsv4/` — decide at lift). Rewrite imports: `bb-rex/.../virtual` →
  `pkg/virtual`; `bb-storage/pkg/filesystem{,/path}` → `internal/bb/...`;
  `go-xdr/...` → `internal/xdr/...`.
- **Shim** `clock` (Clock iface + Now) and `random` (SingleThreadedGenerator) —
  tiny, stdlib-backed (vendor by symbol, NOT `util` wholesale — see ⚠ below).
- **Strip** `metrics_program.go`'s prometheus to no-ops.
- **Replace** `system_authenticator.go` with a trivial localhost AUTH_SYS
  authenticator (no auth/jmespath/eviction).
- **Watch for** the type fork being truly closed (it should be — R2c aligned the
  types) and any *other* bb-storage symbol the server pulls that the floor missed
  (the R2b `x/sys/unix` surprise says: trust `go build`, not the map).

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

Loop step to resume at: **4 (Implement)** for R2d — lift the bb-rex server files
into the tree and rewrite imports onto `pkg/virtual` + `internal/{bb,xdr}`.
(R2a/R2b/R2c done — DEC-010/014/015.)

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
