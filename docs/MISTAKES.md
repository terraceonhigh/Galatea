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
