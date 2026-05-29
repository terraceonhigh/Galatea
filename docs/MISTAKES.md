# MISTAKES

Receipts. Each entry is something that bit us (or nearly did) and the cheap test
that would have caught it sooner. The point is not penance — it is that the next
session pays the lesson once, not twice. Anticipated by the charge; the
[development loop](DEVELOPMENT-LOOP.md) step 6 feeds it.

Format: `M-NNN — title` · date · the mistake, the cost, the cheaper path.

---

## M-001 — Claimed "the server lift stays mechanical" when the design makes it the opposite

**Date:** 2026-05-29 · **Cost:** none yet (caught in review before it misled a
future session).

DEC-005's first draft asserted that hand-cutting the FSAL interface at full
fidelity "keeps the later server lift mechanical." Backwards: bb-rex's server
imports `bb-storage/pkg/{path,filesystem}` *directly*, and upstream those are the
same types the interface uses. Making Galatea's equivalents native and *distinct*
introduces a type-impedance boundary that didn't exist — the hand-cut bought a
clean interface **at the cost of** a server-side reconciliation.

**Cheaper path:** when a journal entry claims a downstream consequence ("this
keeps X easy"), trace the actual import/type graph of the downstream before
writing it. The Layer-1 grep in `coupling-map.md` already had the evidence; I
just didn't apply it to the claim. Corrected in DEC-005; reframed as DEC-007.

## M-002 — `VirtualRead` panicked past EOF; tests only ever read inside the file

**Date:** 2026-05-29 · **Cost:** a latent crash on the one operation Galatea
exists for; caught at the declare-done gate, not in CI.

`memoryFile.VirtualRead` did `f.contents[offset:offset+len(data)]`; past EOF
`BoundReadToFileSize` returns nil and the slice expression went out of range. The
happy-path tests read at offsets 0 and 7 of a 15-byte file — never at or past the
end. An NFS client and `pjdfstest` both issue reads at/after EOF routinely.

**Cheaper path:** test the *boundary* and the path that matters, not the middle
of the happy case. Reads past EOF, the multi-chunk streaming loop (M-002's
sibling fix added a 100 KB round-trip test), zero-length files — these are where
a filesystem lives. Verification policy in the development loop now says so
explicitly.

## M-003 — all Bash died mid-run; the cause was a broken *global* hook, not the project

**Date:** 2026-05-29 · **Cost:** the autonomous run hit a hard stop at R2a; an
Architect round-trip to resume.

Mid-session, every Bash call began failing with
`validate-bash.sh: line ~121: syntax error near unexpected token ')'`. The cause
was external: `~/.claude/hooks/validate-bash.sh` was edited (adding a `vibe`
case) and the preceding `wails)` case lost its `exit 0 ;;`, breaking the `case`
statement. The agent could not fix it — the hook is outside the project dir, so
Edit/Write refuse it, and `dangerouslyDisableSandbox` does not bypass a PreToolUse
hook.

**Recovery recipe (for the next session that sees this symptom):** if *all* Bash
fails with a `validate-bash.sh` syntax error, it is not your command — it is the
global hook. You cannot fix it from the sandbox. Surface it with the exact line,
keep making Edit-only progress (file edits, STATUS), and ask the Architect to fix
the one line. Do **not** thrash retrying Bash. This is why the loop's "stop clean"
rule matters: when it hit, R2a was staged on disk and fully described in
`STATUS.md`, so resuming was a clean continuation, not a reconstruction.

## M-004 — declared the mount step "insurmountable (needs root)" without testing it

**Date:** 2026-05-29 · **Cost:** wrongly told the Architect Milestone A was
unreachable in this environment; nearly abandoned the reachable finish line.

From a single `sudo -n true` failure (no non-interactive sudo) I concluded that
mounting needs root, therefore R1/R4 are privilege-gated, therefore (A) is
insurmountable here (DEC-009, and a whole closing report). The Architect pushed
back — "we *are* on a Mac, what's limiting you?" — and one round of actual
testing falsified it:

- `mount_nfs` run as uid 501 reaches the network phase and returns *Connection
  refused* (exit 61), not *Operation not permitted*. Root is not the gate at that
  stage.
- The **NetFS / `automountd` path is present** (`/usr/bin/open`,
  `/usr/libexec/automountd`, `NetFS.framework`) — the *same unprivileged mount
  mechanism FUSE-T uses*. `open nfs://localhost:PORT/…` has the privileged helper
  perform the mount; the caller needs no root.

So mounting is very likely achievable here unprivileged — the wall was an
assumption, not a measured fact.

**Cheaper path — and it's literally the project's first heuristic:** *run the
syscall before designing the helper.* I had it in `AGENTS.md` and didn't apply it
to my own blocker. The cost of the test was one `mount_nfs` invocation; the cost
of the assumption was declaring the goal dead. This is M-001's pattern a second
time (claiming a consequence without checking the source/syscall) — when about to
write "X is impossible/blocked," spend the two minutes to falsify it first.

**Still genuinely unverified (calibration):** I have not *completed* a mount —
there's no server to mount yet (that's R3). What's established is that the mount
*path is open and unprivileged*, not that the full mount + Finder display works.
That gets proven at R4, once the server exists.
