# STATUS — the cursor

**The single source of "where are we."** First thing a resuming session reads
(see [`DEVELOPMENT-LOOP.md`](DEVELOPMENT-LOOP.md) § Recovery); last thing every
loop updates. If this file and the code disagree, the code is truth — fix this.

---

**Updated:** 2026-05-29 (autonomous run: R1 gated; R2a + serving-foundation done;
(A) confirmed reachable; R2b carve mapped)
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

## Cursor — next increment

**R2b → R2 — Lift the NFSv4 server (DEC-007).** ([`ROADMAP.md`](ROADMAP.md))

> **Done when:** the lifted server package compiles and bb-rex's in-tree server
> tests pass against the in-memory FSAL; `go list` shows no bb-storage import
> outside the vendored floor.

**The type fork is decided (DEC-011):** vendor `path`+`filesystem`, re-point
`pkg/virtual`'s leaf types to them, retire the hand-cut natives. The server then
lifts with import-rewrites only. The R2 gate is re-scoped to "compiles + a
Galatea smoke COMPOUND test" (bb-rex's gomock tests are a separate mountain).

Investigation done — the map for the next session:

- **Server `virtual.*` surface** (nfs40): entirely interface/attributes/status/
  permissions — **all already in `pkg/virtual`.** No handle-allocator or CAS
  types. The server does *not* need the CAS machinery lifted. ✅ big de-risk.
- **Server external deps to handle:**
  - `go-xdr` — vendored (R2a). ✅
  - `path` — **20 files**, imports `grpc/codes`+`grpc/status` (error returns) and
    bb-storage `util`. Vendor + either accept grpc or strip grpc-status.
  - `filesystem` — `FileType*` consts + `DeterministicFileModificationTimestamp`
    (+ `util`, `windowsext` tail). Vendor.
  - `clock` (Clock iface + Now), `random` (SingleThreadedGenerator) — tiny shims.
  - `prometheus` (nfs40 metrics) — strip to no-ops.
  - `auth`/`jmespath`/`eviction` — **don't vendor**; replace `system_authenticator.go`
    with a trivial localhost AUTH_SYS authenticator.

Done this run toward R2: **rpcserver + a self-contained errgroup vendored**
(spike/serving foundation, committed, green). Strategy refined (DEC-013): the
"spike" is realized as **lifting bb-rex read-path-first**, not a from-scratch
server (hand-writing correct FATTR4/READDIR/state-handshake is slower than lifting
the complete impl). So the path forward is R2 proper, sequenced read-first.

**R2b recipe (precise — investigated this run; vendor by copy+strip):**
- `path`: vendor the package into `internal/bb/filesystem/path`. **Drop**
  `local_format_windows.go` (`//go:build windows`). On darwin, 9 files compile and
  use grpc/util: `absolute_scope_walker, builder, component,
  loop_detecting_scope_walker, unix_format, relative_scope_walker,
  virtual_root_scope_walker_factory, trace, windows_format`. path's ONLY external
  symbols are `util.StatusWrap`/`util.StatusWrapf` and `status.Error`+`codes.*` —
  all error wrappers. Strip: `status.Error(codes.X,"m")` → `errors.New("m")`;
  `util.StatusWrap(err,"m")` → `fmt.Errorf("m: %w", err)`;
  `util.StatusWrapf(err,"f",a...)` → `fmt.Errorf("f: %w", a..., err)`; fix imports
  (drop grpc+util, add errors/fmt). Optional shrink: the server uses only
  `{Component,Parser,Format,NewComponent,UNIXFormat,EmptyBuilder,VoidScopeWalker,
  Resolve}` — the absolute/relative/loop_detecting/virtual_root scope-walker
  variants may be deletable if nothing kept references them (verify, Go compiles
  whole-package).
- `filesystem`: survey the same way (needs `FileType*`,
  `DeterministicFileModificationTimestamp`, `DeviceNumber`, `RegionType`,
  `FileInfo`); expect the same grpc/util error-wrapper tail; strip identically.

> **⚠ Do NOT vendor `util` wholesale** — it pulls jsonnet, protobuf, grpc,
> prometheus, uuid. The "8-package floor" (coupling-map) is *symbol*-light but
> *transitive-dependency*-heavy: vendor only the symbols actually used (here,
> nothing from util survives the error-wrapper strip). Same caution for any floor
> package taken whole.

- **R2c** — execute DEC-011: re-point `pkg/virtual` leaf types to the vendored
  `path`/`filesystem`; fix `pkg/osfs`, the in-memory FSAL, `cmd/galatea`; re-verify.
- **R2d** — copy the server files (`nfs40_program`, `nfs41_program`,
  `opened_files_pool`, `minor_version_fallback`, `metrics_program` with prometheus
  stripped, a localhost `system_authenticator`), rewrite imports, compile; smoke
  COMPOUND test vs the in-memory FSAL.
- **R3/R4** — wire `cmd/galatea serve` (rpcserver loop, NFS prog 100003), then
  `open nfs://localhost:PORT/` to mount read-first.

Loop step to resume at: **4 (Implement)** for R2b — the carve is fully mapped.

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
