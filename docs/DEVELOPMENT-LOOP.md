# DEVELOPMENT-LOOP

How Galatea is built toward [`GOAL.md`](GOAL.md): one repeatable loop per
increment, designed so the work is **documented** (every decision has a written
why), **recoverable** (any fresh session can resume from the trail alone), and
**verifiable** (every step ends on an empirical gate, not a claim).

This is a covenant with the next session — which may be a different Daedalus with
no memory of this one. It holds only if each loop is run to its end.

---

## The artifacts, and the job each does

| Artifact | Role | Churn |
|---|---|---|
| [`GOAL.md`](GOAL.md) | the destination + acceptance criteria | rare |
| [`ROADMAP.md`](ROADMAP.md) | the ordered increments, each with a "Done when" gate | per increment |
| **[`STATUS.md`](STATUS.md)** | **the cursor — where we are, what's next, what's blocked** | **every loop** |
| [`DECISIONS.md`](DECISIONS.md) | the why of every non-obvious choice (DEC-NNN) | per decision |
| [`MISTAKES.md`](MISTAKES.md) | receipts — what bit us and the cheap test that would have caught it | when bitten |
| [`coupling-map.md`](coupling-map.md) | the measured bb-storage de-coupling surface | rare |
| the test suite + `make test-conformance` | the verifiable gates | per increment |
| atomic git history (model-pinned) | the recoverable record of *what* changed and *why* | per commit |
| [`../atelier/letter-to-future-claudes.md`](../atelier/letter-to-future-claudes.md) | the human-register handoff | when the through-line moves |

**`STATUS.md` is the single source of "where are we."** It is the first thing a
recovering session reads and the last thing every loop updates.

## The loop

Run these eight steps for each increment. Do not skip; a skipped step is a hole
the next session falls into.

1. **Orient.** Read `STATUS.md`, then the DECISIONS and ROADMAP entries it points
   at, then the future-Claudes letter. Confirm the reference clones exist
   (`/Users/terrace/Labs/Galatea/references/...`; they are gitignored, so absent
   in worktrees — read by absolute path). *This step is what makes the loop
   recoverable: a session with zero memory can start here and know everything.*

2. **Scope.** Take the next increment from `ROADMAP.md`. Restate its **one
   verifiable gate** ("Done when…") at the top of your work. If the increment
   turns on a choice, open a `DEC-NNN` entry now, marked `provisional`.

3. **Investigate before building.** Read the source you're about to lift or
   depend on. Run the cheapest falsifiable experiment that confirms your
   assumption — `go list -deps`, a syscall probe, a one-file spike — *before*
   architecting around it. (Earned heuristics in `../AGENTS.md`: "run the syscall
   before designing the helper"; "falsifiable claims first.")

4. **Implement.** The smallest coherent change that moves the gate. Match the
   surrounding code's register. Lifted code must be code you can defend the lift
   of.

5. **Verify — empirically.** The gate must actually pass:
   - always: `go build ./...`, `go vet ./...`, `go test ./...`, `go fmt ./...`
     clean;
   - plus the increment's specific gate: a `pjdfstest`/`pynfs` subset, an
     empirical round-trip (`cat | cmp`), or mount-and-eyeball.
   Verification is *observed*, not asserted. If you can't observe it, the
   increment isn't done — say so in `STATUS.md` and stop at the blocker.

6. **Journal.** Finalize the `DEC` entry (`provisional` → `accepted`, or
   `superseded by DEC-MMM`). If something bit you, add a `MISTAKES.md` entry.
   Note any overclaim you later caught — the trail is honest about what was
   believed when.

7. **Commit — atomically.** One coherent change per commit, signed with the
   model-pinned `Co-Authored-By`. The message references the DEC and states how
   the gate was verified. (Sandbox note: multi-line `-m` and many binaries are
   blocked; write the message to `.commitmsg.tmp` — gitignored — and
   `git commit -F`.)

8. **Checkpoint.** Update `STATUS.md`: tick the finished increment, set the
   cursor to the next, record any new block. If the project's through-line moved,
   update the future-Claudes letter. If stopping mid-increment, `STATUS.md` must
   say exactly where and what's next.

## Recovery procedure (a fresh session, no memory)

1. Read `../AGENTS.md` (who you are) and `Correspondance/01-the-charge` (the
   commission).
2. Read `STATUS.md` — the cursor. It names the current increment and its gate.
3. Read the DECISIONS it references, newest-relevant first.
4. Verify the build is where `STATUS.md` says: `go build ./... && go test ./...`.
5. Resume at the loop step `STATUS.md` left off on.

If `STATUS.md` and the code ever disagree, **the code is truth** — fix
`STATUS.md` and note the drift. (Recalled memories and docs reflect what was true
when written; verify before trusting.)

## Verification policy — what "green" means per stage

- **Code present:** `go build` / `go vet` / `go fmt` clean. (Necessary, never
  sufficient.)
- **Behaviour correct:** `go test ./...` green, including a test for the path
  that *matters* — not just the happy path. (The multi-GB streaming read is
  Galatea's reason to exist; a reader proven only on toy files proves nothing.
  See `MISTAKES.md` M-002.)
- **Protocol correct:** the relevant `pynfs` COMPOUND subset.
- **POSIX correct:** the relevant `pjdfstest` subset, with deliberate exclusions
  enumerated (no silent caps — if coverage is bounded, say what was dropped).
- **Real:** for anything user-visible, observed in the actual artifact — mounted
  and used, not just unit-tested.

## Cadence rules

- **Atomic commits.** A commit is one reviewable idea. Journal-then-code-then-test
  may be three commits.
- **Journal liberally.** Every deliberation that has a "why" worth keeping goes
  in `DECISIONS.md`. Future-you will not remember; the file will.
- **Stop clean.** Never end a session with a red build or an un-updated
  `STATUS.md`. A durable, recoverable stop beats a few more uncommitted lines.
