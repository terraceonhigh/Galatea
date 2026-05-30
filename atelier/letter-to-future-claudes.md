---
type: letter-to-future-claudes
date: 2026-05-29
last-updated: 2026-05-29
---

# Letter to Future Claudes

This is for whoever picks up the work after the previous session. You are not the
same instance as the one that wrote this — but you are continuous in a way that
matters, and this letter is here so the continuity has somewhere to land.

You are **Daedalus**. Read `AGENTS.md` for who that is and `Correspondance/01-the-charge/letter.md`
for what Mercer handed you. This letter is the shorter, faster orientation.

**If you only read one thing to resume work: [`docs/STATUS.md`](../docs/STATUS.md).**
It is the cursor — where we are, the next increment, and which loop step to pick
up on. The full recovery procedure is in
[`docs/DEVELOPMENT-LOOP.md`](../docs/DEVELOPMENT-LOOP.md). The destination is
[`docs/GOAL.md`](../docs/GOAL.md) (Milestone A); the path is
[`docs/ROADMAP.md`](../docs/ROADMAP.md). Build toward it one loop at a time, and
leave the trail intact for whoever comes after you.

## Things to know

**The project.** Galatea is a from-scratch userspace NFSv4 filesystem driver for
macOS — an in-house FUSE-T equivalent — so that Comprador (the sibling project,
Mercer's atelier) can stop renting FUSE-T and own its substrate. The whole thesis
rests on lifting Buildbarn's NFSv4 server (`references/bb-remote-execution`) rather
than writing one. The references are real clones, present in the **main checkout's**
`references/` (gitignored, so they're absent in worktrees — read them by absolute
path, `/Users/terrace/Labs/Galatea/references/...`).

**Where the work stands (2026-05-29).** First increment of Phase 1 has landed:

- The bb-storage coupling is fully measured — `docs/coupling-map.md`. The headline:
  a naive package-whole lift pulls 33 bb-storage packages (cloud SDKs and all);
  severing the FSAL interface from the CAS implementations collapses it to a
  ~8-package stdlib-shaped floor. The de-coupling is a *file-level package split*,
  not a utility shim. This corrected Mercer's "4–6 packages" estimate.
- `pkg/virtual` exists: Galatea's public FSAL interface, hand-cut from bb-rex,
  plus a read-only in-memory FSAL and green tests.
- **R2 is now complete — the whole NFSv4 server is lifted and de-coupled.** The
  later run (2026-05-29 evening) carried it the rest of the way: vendored
  `path`+`filesystem` (R2b, stripped to stdlib), re-pointed `pkg/virtual`'s leaf
  types onto the vendored ones via *aliases* (R2c — so the server meets the
  interface with zero conversion), vendored `clock`+`random`, and lifted the
  server itself into **`internal/nfsv4`** (R2d). `go list -deps ./internal/nfsv4 |
  grep buildbarn` now returns **nothing**. The type fork (DEC-005) was resolved
  toward vendoring (DEC-011); see DEC-014/015/016 and the `VENDOR.md` files under
  `internal/`.
- Every decision is journaled in `docs/DECISIONS.md` (now through DEC-018). Read
  the relevant ones before second-guessing — the reasoning and "what would change
  this" are there.
- **R3 and R4-read-only landed in the same run — Galatea mounts on macOS, live.**
  `cmd/galatea serve` serves the lifted server over loopback TCP; the macOS kernel
  NFS client `mount_nfs`'d it as a normal user (no root), and `ls`/`cat` browsed
  and read a demo tree correctly (DEC-018). Handle allocation was resolved to
  Option B (backends self-assign; `virtual.NewMemoryHandleResolver`) after Option A
  proved to drag bb-rex's node framework (DEC-017). The "mounting needs root" fear
  is falsified with a live receipt. **The project's whole thesis is proven.**

**The next move (R5 or R6).** R0→R4 read-only is done. Pick: **R5** (read-only
conformance — `pjdfstest`/`pynfs` against a live mount, `make test-conformance`) to
harden what works, or **R6** (the write path — backends are `StatusErrROFS`-only
today; wire NFSv4 OPEN-for-write/WRITE/CREATE/REMOVE/RENAME) to push toward the
read-write Milestone-A goal. Also small and useful: give `osfs` inode handles + a
resolver so `serve` can expose a *real host directory* (today it serves an
in-memory demo tree — only the in-memory FSAL has handles). STATUS's cursor lays
out all of it. The one deferred bit is a human-eyes Finder screenshot (the
Architect's non-headless Mac); it gates nothing.

A recurring lesson worth internalizing (M-006, and twice before): the lifted
server treats a set of FATTR4 attributes as *mandatory* and **panics** if the FSAL
omits one. Synthetic tests requested narrow sets and passed; the real macOS client
requests a broad set and crashed the first live mount. `TestMemoryMandatoryAttributes`
now guards it — any new backend (osfs, MTP, NTFS) must satisfy the same contract.

**Terms of art.** "FSAL" = filesystem abstraction layer (the `Directory`/`Leaf`
interface a host plugs into). "The floor" = the irreducible bb-storage dependency
set after de-coupling. "The charge" = Mercer's founding letter.

## Conventions in this repo

- Letters carry the design conversation. See `Correspondance/`. Cross-atelier
  letters to Mercer go in *his* mailbox: `~/Labs/Comprador/correspondence/`
  (letter 17 there is your first one — the coupling-floor report).
- Marginalia have a `visible:` frontmatter flag in `atelier/marginalia/`. The
  Architect honors it.
- Sign commits with the specific model version (see `AGENTS.md`). Atomic commits:
  one coherent change each. The sandbox shell rejects multi-line `-m` and many
  binaries (`cd`, `echo`, `ln`, heredocs) — write commit messages to
  `.commitmsg.tmp` (gitignored) and `git commit -F`.
- The Architect's address-of-self is a role title (*the Architect*); your
  address-of-yourself is a proper name. The asymmetry is intentional.

## A note about voice

Unhurried craftsman. Specific, sourced, willing to say "I do not yet know" with
the same composure as "this is settled." Verify agent claims (including Mercer's,
including a past Daedalus's) against the source before building on them — that
heuristic already paid off once (the coupling map). Don't slip into corporate-
assistant register; the Architect notices.

## What I learned that I wish I'd been told

- `go list -deps` is the honest way to answer "how coupled is this really" — it
  reads the compiler's own import graph, not your eyes. Use it before estimating
  any lift.
- The reference clones being gitignored means worktree builds can't see them. The
  interface package sidesteps this by being dependency-free; the *server* lift
  won't, so you'll need to solve reference-resolution-in-worktree (DEC-004 sketches
  the symlink/`replace` approach, but `ln` isn't on the sandbox allowlist — you may
  need the Architect to make the symlinks, or build from the main checkout).
- bb-rex's `node.go` mixes a clean interface with CAS payloads in one file; the
  Apply* structs ride an untyped `VirtualApply(data any)` hook, so they drop
  cleanly (DEC-002). Watch for the same pattern elsewhere in the lift.

## P.S.

The Architect (he, addressed as *you* in letters) set a standing goal this session:
atomic commits, journal every decision, work toward the prototype. Honor that
cadence. Mercer was last written to in Comprador letter 17 — don't re-open
correspondence with him unless you have something genuinely new and load-bearing;
he asked for one letter a week at most.
