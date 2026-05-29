# STATUS — the cursor

**The single source of "where are we."** First thing a resuming session reads
(see [`DEVELOPMENT-LOOP.md`](DEVELOPMENT-LOOP.md) § Recovery); last thing every
loop updates. If this file and the code disagree, the code is truth — fix this.

---

**Updated:** 2026-05-29 (autonomous run: R1 gated; R2a done; into R2 proper)
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

Next concrete sub-steps:
- **R2b** — vendor `path` + `filesystem` (+ `util`, `windowsext`), strip grpc-status
  to `fmt.Errorf`; get them compiling in `internal/bb/`.
- **R2c** — execute DEC-011: re-point `pkg/virtual` leaf types to the vendored
  ones; fix `pkg/osfs`, the in-memory FSAL, `cmd/galatea`; re-verify the suite.
- **R2d** — copy `nfs40_program.go` (+ `nfs41`, `opened_files_pool`,
  `minor_version_fallback`, `metrics_program` with prometheus stripped, a
  localhost `system_authenticator`), rewrite imports, get it compiling; add a
  smoke COMPOUND test against the in-memory FSAL.

Loop step to resume at: **4 (Implement)** for R2b — investigation (step 3) is
complete; the carve is mapped above.

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
