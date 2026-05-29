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

> **Update 2026-05-29:** the `node.go` unknown is resolved — see DEC-002. It
> splits cleanly. The strategy stands.

---

## DEC-002 — `node.go` splits cleanly: keep the `Node` interface, drop the `Apply*` payloads

**Date:** 2026-05-29 · **Status:** accepted

**Decision.** The one unclassified file from DEC-001, `virtual/node.go`, is not a
blocker. Galatea keeps the `Node` interface and the `GetFileInfo` helper verbatim
and **drops all five `Apply*` payload structs**. Doing so removes node.go's
`blobstore`, `digest`, `bazeloutputservice`, and `outputpathpersistency` imports
entirely.

**Evidence** (read of `node.go` in full, bb-rex `ed02b7a`):

- `node.go` defines (a) the `Node` interface — the intersection embedded by both
  `Directory` and `Leaf`; (b) `GetFileInfo`; and (c) five `ApplyXxx` structs.
- The `Node` interface's four method signatures —  `VirtualGetAttributes`,
  `VirtualSetAttributes`, `VirtualApply(data any) bool`,
  `VirtualOpenNamedAttributes` — reference only `context`, `AttributesMask`,
  `Attributes`, `Status`, `Directory`. **No CAS types.**
- The CAS coupling lives entirely in the `Apply*` structs (`ApplyUploadFile`
  carries `blobstore.BlobAccess` + `digest.Digest`; `ApplyGetContainingDigests`
  carries `digest.Set`; two more carry Bazel-output-service protos). These are
  *payloads* passed through `VirtualApply(data any)` — an untyped, type-switched
  extension hook. They are bb-rex's CAS/Bazel features, not part of any interface
  signature.
- Because `VirtualApply` takes `any`, dropping the structs is invisible to the
  interface contract: a Galatea FSAL simply never receives those payloads and
  `VirtualApply` returns `false` for them. `GetFileInfo` is already CAS-free.

**Why this matters.** DEC-001's clean-split claim rested on `node.go` being
interface-bearing rather than CAS-implementation. It is *both* in the same file —
but the two concerns are textually separable with a knife, not a scalpel. The
interface half is exactly what Galatea must expose; the payload half is exactly
what it must shed. No type from the dropped half leaks into the kept half.

**What would change this.** If a future downstream wants CAS-backed files through
Galatea (it won't — Comprador's backend is MTP, not content-addressed), the
`ApplyUploadFile` payload would need reinstating, re-importing `blobstore`/
`digest`. Out of scope by design.

**Consequence for the extraction.** The kept surface of `node.go` is ~40 lines
(interface + `GetFileInfo`). When the `virtual` interface package is carved, this
is one of the first files in, trimmed of its lower two-thirds.

---

## DEC-003 — Module path `github.com/terraceonhigh/galatea`

**Date:** 2026-05-29 · **Status:** provisional (no remote chosen yet)

**Decision.** The Go module is `github.com/terraceonhigh/galatea`, lowercase per
Go import idiom. Chosen to match the GitHub org used by sibling `Foral`
(`github.com/terraceonhigh/Foral`) and Comprador.

**What would change this.** If Galatea lands on Forgejo (`forge.terrace.zone`)
instead — plausible, since the house keeps private/personal repos there — the
path changes to match. Cheap to rename now (no importers); journaled so the
choice is visible rather than silent. Revisit when the Architect picks the host.

---

## DEC-004 — Build against the local `references/` clones via `replace`, not the module proxy

**Date:** 2026-05-29 · **Status:** accepted (for the lift phase)

**Decision.** `go.mod` pins bb-rex, bb-storage, and go-xdr with `replace`
directives pointing at the local reference clones rather than fetching
pseudo-versions from `proxy.golang.org`. In a worktree — where `references/*` is
gitignored and therefore absent — the clones are made resolvable with local
symlinks into the main checkout; the symlinks are themselves gitignored, so they
never enter the tree.

**Why.**
- The reference clones are the exact source I read and measured (bb-rex
  `ed02b7a`). Building against them, not a proxy fetch, guarantees the code I
  reasoned about is the code I compile. No version skew.
- bb-storage's go.mod pseudo-version is future-dated (`v0.0.0-20260507...`); the
  proxy may not carry it, and the network is not a dependency I want in the lift
  loop.
- `replace` is the standard Go mechanism for exactly this; it leaves `require`
  lines honest about provenance.

**Relationship to DEC-001.** This is a *scaffolding* decision, not a reversal of
DEC-001. The end-state module is bb-storage-free. Depending on the floor now —
measured and bounded at 8 packages — gets a green build fast; the shim work
replaces those imports incrementally afterward. Nothing is hidden: the coupling
is already mapped.

**What would change this.** Once the 8-package floor is shimmed/vendored in-tree,
the bb-storage `replace`/`require` lines are deleted. The go-xdr line persists
(lifted as-is); the bb-rex line goes once the server files are copied in.
