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
  **bb-storage-free**, plus a read-only in-memory FSAL and green tests.
- Every decision is journaled in `docs/DECISIONS.md` (DEC-001 … DEC-005). Read
  them before second-guessing a choice — the reasoning and the "what would change
  this" are there.

**The next move** is the big one: lift bb-rex's NFSv4 server (`nfs40_program.go`,
~112 KB) and reconcile it with `pkg/virtual`. The unresolved fork (DEC-005): the
server's method signatures use bb-storage's `path.Component`/`filesystem.FileType`,
but our interface uses Galatea-native types — so either sed-rewrite the server's
types, or keep them and adapt at the boundary. Decide that *with the server code in
front of you*, and journal it as DEC-006.

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
