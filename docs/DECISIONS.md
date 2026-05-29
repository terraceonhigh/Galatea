# DECISIONS

Galatea's decision journal. One entry per decision that future-Daedalus (or the
Architect, or Mercer) would otherwise have to reverse-engineer from the code.
Append-only in spirit: supersede with a new entry rather than editing an old one,
so the trail stays honest about what was believed when.

Format: `DEC-NNN — title` · date · status (`accepted` / `superseded by DEC-MMM` /
`provisional`). State the decision, then the evidence, then what would change it.

---

## DEC-001 — De-couple by severing the FSAL interface from the CAS implementations, not by shimming a fixed list of bb-storage utilities

**Date:** 2026-05-29 · **Status:** provisional (measured, not yet executed)

**Decision.** Phase 1's de-coupling is reframed. The job is *not* "vendor or shim
the 4–6 bb-storage utility packages nfsv4 imports" (the framing in Mercer's kit
shopping list). It is: **lift only the FSAL interface and server logic out of
bb-rex's `pkg/filesystem/virtual`, leaving the content-addressed-store (CAS) FSAL
implementations behind in `references/`.** Once that cut is made, the heavy
bb-storage surface collapses on its own.

**Evidence** (compiler-grounded — `go list -deps`, bb-rex @ `ed02b7a`, go 1.26.3;
full method and tables in [`coupling-map.md`](coupling-map.md)):

- Naive `go list -deps ./pkg/filesystem/virtual/nfsv4` pulls **33** bb-storage
  packages — including `cloud/aws`, `cloud/gcp`, `blobstore`, `blockdevice`,
  `otel`, `zstd`, and a dozen `proto/configuration/*`. That is far past the
  estimate of 4–6 utility packages.
- The explosion is entirely *transitive through the parent `virtual` package's
  CAS FSAL implementations*. The nfsv4 production files' **direct** bb-storage
  surface is only 7 packages (`random`, `filesystem/path`, `filesystem`,
  `clock`, `jmespath`, `eviction`, `auth`). None of them import
  `blobstore`/`digest`/`cloud` directly.
- The CAS coupling inside `virtual` is **file-localized**: 8 files named
  `cas_*` / `*_cas_*` / `node.go` carry the `blobstore`/`digest` imports;
  `directory.go` and `leaf.go` (the interface we must expose) are CAS-free.
- The `auth` + `jmespath` subtree (which drags in `digest`, `otel`, `program`,
  `proto/auth`, `proto/configuration/{digest,jmespath}`) is confined to a single
  file, `system_authenticator.go`. Replacing it with a trivial localhost
  authenticator drops that whole tail.
- Floor after both cuts (sever CAS, strip auth/jmespath): **8** bb-storage
  packages, all genuinely stdlib-shaped utilities — `clock`, `random`,
  `eviction`, `filesystem`, `filesystem/path`, `util`, + `proto/configuration/`
  `{eviction,tls}`. *This* is the real shim target, and it sits at or below the
  low end of the 500–2,000 LOC estimate.

**What this changes.** The hard part of the de-coupling is not shim volume — it's
the *surgical file-level split of the `virtual` package* (Go compiles packages
whole, so the interface and the CAS implementations must be physically separated
into distinct packages). The utility shim underneath is comparatively small.

**What would change this decision.** (1) `node.go` is the one unclassified file —
if the FSAL interface turns out to need types defined in a CAS-coupled `node.go`,
the clean split is harder than measured. (2) If a downstream eventually wants
JMESPath authorization, the `auth` strip reverses. (3) The numbers are a
*dependency floor*; the cut has not been executed, so the LOC of the shim layer
is still projected, not weighed.

**Next action.** Investigate `node.go`'s role (interface-bearing vs.
CAS-implementation). Then prototype the `virtual`-interface extraction into a
fresh package and confirm it compiles against an in-memory FSAL with only the
8-package floor present.
