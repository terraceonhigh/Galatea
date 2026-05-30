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

> **Update 2026-05-29:** DEC-005 supersedes the build approach for the *interface
> package*. The interface is hand-cut and stdlib-replaced — it needs no bb-storage
> dependency at all, so DEC-004's `replace` machinery is not exercised by the first
> increment. DEC-004 still governs the eventual *server* lift, which does import
> bb-rex.

---

## DEC-005 — First increment is a hand-cut, standalone interface package — stdlib-replaced leaf types, full-fidelity contract

**Date:** 2026-05-29 · **Status:** accepted

**Decision.** The first prototype increment is `pkg/virtual` — Galatea's public
FSAL interface, **transcribed by hand from bb-rex's `virtual` package** rather
than imported from it. The `Node`/`Directory`/`Leaf` interface contracts and the
`Status`/`Attributes`/`Child` support types are reproduced at **full fidelity**
(every method, every status code, every attribute). The handful of bb-storage
*leaf types* the contract touches — `path.Component`, `path.Parser`,
`filesystem.{FileType,Permissions,RegionType,DeviceNumber,FileInfo}` — are
**replaced with Galatea-native equivalents** in the same package.

This realizes DEC-001's "stdlib-replaced" option for the interface, and satisfies
the Phase-1 success criterion directly: *the interface is clean of bb-storage
dependencies.* The package compiles with zero external dependencies.

**Why hand-cut, not import:**
- The Phase-1 criterion requires the exposed interface be bb-storage-free.
  Importing bb-rex's `virtual` package leaves it bb-storage-*dirty*. Only a
  transcription (or a vendor-and-rewrite) satisfies the criterion.
- The interface is Galatea's actual product surface — the thing every host plugs
  into. It is the right thing to own line-by-line first, before the engine.
- It needs no network, no module-graph resolution, no `replace` gymnastics. It
  compiles standing alone.

**Why full-fidelity (not a reduced subset):** a reduced interface would drift
from what bb-rex's server expects, turning the eventual server lift into a
redesign. Reproducing the exact method set keeps the interface *shape* stable —
the server calls the same method names with the same argument structure.

**The deferred consequence — stated plainly, and a correction.** Do not read the
paragraph above as "the server lift is mechanical." It is not, and an earlier
draft of this entry wrongly implied so (caught in review). bb-rex's nfsv4 server
imports `bb-storage/pkg/filesystem/path` and `pkg/filesystem` *directly* (see the
Layer-1 surface in `coupling-map.md`), and upstream those are the **same** types
the `virtual` interface uses. By making `virtual.Component`/`FileType` etc.
Galatea-native and *distinct*, this decision deliberately introduces a
type-impedance boundary that did not exist in bb-rex. The hand-cut bought a clean,
bb-storage-free interface **at the cost of** a server-side type reconciliation.

So when the server is lifted, the fork is:
- (a) **Hand-cut's natural sequel — reconcile:** a sed-pass rewrites the server's
  bb-storage leaf types to Galatea's throughout, or an adapter bridges at the
  boundary. Non-trivial; the price of this decision.
- (b) **The genuinely mechanical alternative — vendor:** copy bb-storage's `path`
  and `filesystem` packages (two of the 8-package floor) into the Galatea module
  with an import-path rewrite, keeping the types *identical* to what the server
  expects. Then the server imports them unchanged and compiles with no type churn.

This is the vendor-vs-shim fork from DEC-001, now sharply scoped. It is **not
decided here** — DEC-006 decides it with the server code in hand. Worth weighing
honestly then: hand-cutting the interface may have optimised the wrong stage. If
(b) is chosen for the server, it may be cleaner to *also* back the interface's
leaf types with the vendored packages, retiring the native types from DEC-005.

**What would change this.** If transcription fidelity proves too costly to
maintain against bb-rex upstream drift, switch to vendoring bb-rex's `virtual`
package wholesale with a mechanical import-path rewrite (`buildbarn/bb-storage`
→ Galatea-internal shims). The hand-cut version is the bet that the interface is
small and stable enough that owning it outright is cheaper than tracking it.

---

## DEC-006 — Deliver a CLI-drivable product now: a local-filesystem backend + `galatea` navigator. Defer the NFS server lift.

**Date:** 2026-05-29 · **Status:** accepted

> Numbering note: DEC-005 forecast that "DEC-006 decides the server-lift fork."
> That fork is now **DEC-007** (when the server is lifted). This DEC-006 takes a
> different, smaller step first, in response to the Architect's ask for a working
> prototype bounded by what's drivable in CLI today.

**Decision.** Before lifting bb-rex's NFSv4 server, ship something runnable:

1. `pkg/osfs` — a **read-only FSAL backed by the local OS filesystem**
   (`os.ReadDir`/`os.Open`/`os.Stat` → `virtual.Directory`/`virtual.Leaf`). This
   is Galatea's *second* backend, and the first backed by something real.
2. `cmd/galatea` — a CLI that roots an `osfs` FSAL at a host directory and drives
   it (`ls`, `cat`, `stat`, `tree`). Every operation goes **through the
   `virtual.*` interface** — `VirtualLookup`, `VirtualReadDir`, `VirtualRead`,
   `VirtualGetAttributes` — i.e. the same calls an NFS server would make. The CLI
   stands in for the NFS client a future mount will provide.

**Why this, why now.** The product's defining feature — a Finder-visible NFS
mount — needs the 112 KB server lift *and* root privileges for `mount_nfs`,
neither of which lands in a day. But the FSAL is the load-bearing abstraction,
and a second real backend plus a driver proves it end-to-end with zero
privileges, entirely in CLI. It is a rudimentary product: you point Galatea at a
directory and operate on it through Galatea's own filesystem layer.

**What it deliberately is not.** No NFS wire protocol, no mount, no write path.
The local backend is read-only (matching the in-memory one) so the increment
stays bounded and honest.

**What this validates.** The charge asked that the interface "be designed to
support" real backends. Until now the only implementer was the in-memory test
FSAL, which can be shaped to fit the interface. A backend over a real,
externally-defined data source (the OS filesystem) is the first genuine pressure
test of whether the interface is implementable without contortion.

**What would change this.** Nothing reverses it — `osfs` is a permanent reference
backend and a useful test fixture. The next increment (DEC-007) builds *up* from
here: lift the NFS server and serve either backend over the wire.

---

## DEC-008 — Adopt Milestone A as the current ultimate goal, and a fixed development loop to reach it

**Date:** 2026-05-29 · **Status:** accepted

**Decision.** Two ratifications, at the Architect's instruction:

1. **The goal is Milestone A** — a read-write, POSIX-reasonable, Finder-visible
   filesystem of our own, no kext, no closed-source/commercial dependency,
   consumed via `galatea.Mount(...)`. Defined with acceptance criteria in
   [`GOAL.md`](GOAL.md); the ordered path in [`ROADMAP.md`](ROADMAP.md). This is
   the bounded target — explicitly *not* the libfuse C ABI (goal B) or the MTP
   backend (Phase 4). When all of GOAL.md's AC1–AC7 pass, the goal is redefined.

2. **The process is the loop** in [`DEVELOPMENT-LOOP.md`](DEVELOPMENT-LOOP.md):
   orient → scope → investigate → implement → verify (empirically) → journal →
   commit (atomically) → checkpoint. Backed by a recovery procedure and a
   verification policy, anchored on [`STATUS.md`](STATUS.md) as the single cursor.

**Why ratify in the journal.** The goal and the method are themselves decisions a
future session would otherwise have to reconstruct. This entry is the index:
GOAL = destination, ROADMAP = path, STATUS = position, DEVELOPMENT-LOOP = method,
DECISIONS = why, MISTAKES = receipts.

**Why these criteria are durable.** The goal is defined by *observable* gates
(mounts, byte-identical transfer, pjdfstest/pynfs subsets), not by feeling done.
The loop makes every increment documented (a DEC), recoverable (STATUS + the
trail), and verifiable (an empirical gate) — the three properties the Architect
named.

**What would change this.** Reaching Milestone A (redefine the goal toward B);
or empirical evidence at R1 that the NFSv4 substrate doesn't dodge the timeout
class — which would invalidate the architecture under the goal and force a
rethink before R2. That is exactly why R1 is first.

---

## DEC-009 — R1 is privilege-gated in this environment; re-slice to build R2+R3 first, accept the bounded risk

**Date:** 2026-05-29 · **Status:** accepted

**Decision.** R1 (measure that NFSv4 dodges the macOS NFS-client RPC-timeout
class) cannot be run here. It requires a real macOS *kernel* NFS mount, which
requires root; this session is uid 501 with no non-interactive sudo. A userspace
NFSv4 client (pynfs) cannot stand in — the timeout is a property of the kernel
client, not the protocol. So R1 is **blocked, gated on the Architect / a
privileged real-Mac context.**

Rather than stop the whole run at the first privileged wall, **re-slice**: build
**R2 (server lift)** and **R3 (serve over a loopback socket, drive with a
userspace NFSv4 client)** now — both are pure-userspace, no-privilege, and
CLI-verifiable — and leave R1 for the Architect to close before R4.

**The risk this accepts, stated plainly.** R2/R3 are built before R1 confirms the
substrate bet. If R1 later fails (NFSv4 *doesn't* dodge the timeout), that work
is not wasted — the lifted server and its conformance still stand — but the
*architecture* under Milestone A would need a rethink. The risk is bounded and
the Architect can adjudicate it.

**Interim evidence (not a substitute for R1).** FUSE-T, which uses an internal
NFSv4 server mounted by the same macOS client, demonstrably serves the multi-GB
workloads that stalled Comprador's NFSv3 path. That is circumstantial but
directionally strong: it is the existence proof that *an* NFSv4-over-loopback
mount handles these transfers on macOS. R1 upgrades this from "FUSE-T does it" to
"we measured ours."

**Where the wall really is.** R4 (kernel mount → Finder volume) is the hard
environmental ceiling for an unprivileged headless agent: it needs root to mount
and a GUI to confirm Finder visibility. This run's realistic terminus is the end
of R3 — a lifted, de-coupled userspace NFSv4 server proven to answer real
COMPOUNDs over the wire against the osfs backend. That is the engineering core of
(A), minus the privileged mount.

> **CORRECTION 2026-05-29 (see M-004).** The "R4 needs root, insurmountable here"
> claim above is **wrong** — asserted without testing. Measured since: `mount_nfs`
> as uid 501 returns *Connection refused*, not *permission denied*, and the
> NetFS/`automountd` path (FUSE-T's own unprivileged mount mechanism) is present
> (`open`, `automountd`, `NetFS.framework`). R4 is feasible unprivileged via
> `open nfs://localhost:PORT/…`; Finder visibility is verifiable by the Architect
> plus `mount`/`df`. **(A) is reachable on this Mac.** R1 likewise becomes testable
> once a server exists. The genuine remaining work is building the server (R2→R3),
> not privilege. The re-slice in this DEC still stands (build R2+R3 first); only its
> "insurmountable" framing is retracted.

---

## DEC-010 — Vendor go-xdr by copy + import-rewrite into `internal/xdr/` (R2a)

**Date:** 2026-05-29 · **Status:** accepted

**Decision.** go-xdr (the XDR codec and pre-generated NFSv4/RPCv2/darwin wire
stubs, "used as-is" per the charge) is brought into Galatea by **copying the
needed packages into `internal/xdr/` and rewriting their import prefix**, rather
than depending on it via go.mod.

**Why copy, not `require`.**
- The worktree can't see the gitignored `references/` clone, so a local
  `replace` wouldn't resolve here; a proxy fetch would pull go-xdr's dev-tool
  dependency graph (antlr, gomock, gofumpt) into go.sum for code we don't run.
- Copying makes the codec part of Galatea — no external dependency — satisfying
  Milestone A's AC7 (purity) directly. The runtime + stubs depend only on the
  standard library, so the vendored copy is self-contained.

**Scope.** Vendored: `pkg/runtime`, `pkg/protocols/{rpcv2,nfsv4,
darwin_nfs_sys_prot}`. Dropped: all `*_test.go` (dev-dep heavy) and every unused
protocol. `pkg/rpcserver` deferred to R3 (it needs `golang.org/x/sync/errgroup`).
Two logged modifications (import paths; five `%d`→`%v` bool-discriminant format
fixes) — see `internal/xdr/VENDOR.md`. Apache-2.0 `LICENSE` carried alongside.

**Verified.** `go build ./...`, `go vet ./...`, `go test ./...` all green; an
`internal/xdr/smoke` test round-trips a primitive (unsigned hyper) and a generated
NFSv4 enum (`NfsFtype4`), proving the vendored copy encodes/decodes in-tree.

**What this unblocks.** R3's RPC serving needs these wire types; R2's server lift
needs them too. This is the foundation both build on.

---

## DEC-011 — Resolve the type fork: vendor bb-storage `path`+`filesystem`, retire `pkg/virtual`'s native leaf types (supersedes part of DEC-005)

**Date:** 2026-05-29 · **Status:** accepted

**Decision.** The DEC-005/DEC-007 fork is resolved in favour of the **mechanical
(vendor) path**, not the reconcile path. Vendor bb-storage's `path` and
`filesystem` packages into Galatea (copy + import-rewrite, as in DEC-010), and
**re-point `pkg/virtual`'s leaf types to the vendored types** — `path.Component`,
`path.Parser`, `filesystem.{FileType,DeviceNumber,RegionType,FileInfo}` — retiring
the hand-cut natives introduced in DEC-005. `Permissions`, `Status`, `Attributes`,
`Child`, and the `Node`/`Directory`/`Leaf` interfaces stay as they are.

**Evidence (the Explore survey of the four server files):**
- The server uses path's **full machinery**, not just `Component`:
  `path.{Component, Parser, Format, NewComponent, UNIXFormat, EmptyBuilder,
  VoidScopeWalker, Resolve}` — the parser/scope-walker stack, used in
  `pathParserToLinktext4` for symlink-target conversion. Rewriting that to native
  types means *reimplementing path parsing* — large and bug-prone. Vendoring keeps
  it intact.
- The server's `filesystem` use is just the `FileType*` constants +
  `DeterministicFileModificationTimestamp` — small, but they must match
  `pkg/virtual`'s `FileType`, which is the whole point of unifying on the vendored
  type.
- With `pkg/virtual` and the server both on the vendored `path`/`filesystem`
  types, the server's `virtual.*` references resolve to `pkg/virtual` via a pure
  import-rewrite, with **zero type conversion at the interface boundary.**

**Why this is worth reworking already-committed code.** DEC-005 bet that owning
the leaf types outright was cheaper than tracking them. The server survey
falsified that for the *server*: the reconcile cost is reimplementing path
parsing, far more than the vendoring cost. The hand-cut interface still did its
job — it proved the FSAL shape and shipped R0's product — but the leaf types now
swap to the vendored ones. DEC-005's structure stands; its leaf-type sourcing is
superseded here.

**Cost / blast radius.** `pkg/virtual` (types.go retired; attributes.go's
`symlinkTarget` back to `path.Parser`, `deviceNumber`/`fileType` to
`filesystem.*`), plus `pkg/osfs`, `pkg/virtual`'s in-memory FSAL, and
`cmd/galatea` re-typed to the vendored leaf types. Mechanical; each re-verified by
the existing test suite.

**Re-scoped R2 gate.** bb-rex's in-tree server tests are gomock-generated against
mocked interfaces (the `nfs40_program_test.go` alone is 233 KB and needs the
upstream `internal/mock` + gomock). Vendoring that test mountain is its own
project. So R2's gate is re-scoped from "bb-rex's tests pass" to **"the lifted
server compiles, and a Galatea-authored smoke test drives a COMPOUND (e.g.
PUTROOTFH+GETATTR) against the in-memory FSAL"** — a stronger, self-owned check,
folded into R3.

**Also stripped during the lift:** `nfs40_program.go`'s `prometheus` metrics
(replaced with no-op counters or removed) and `system_authenticator.go`'s
`auth`/`jmespath`/`eviction` use (replaced by a trivial localhost AUTH_SYS
authenticator) — so none of prometheus, auth, jmespath, or eviction is vendored.

---

## DEC-012 — Spike a minimal read-only NFSv4 server to a real mount first, then lift bb-rex (Architect-chosen)

**Date:** 2026-05-29 · **Status:** accepted

**Decision.** Before the full bb-rex lift (R2), build a **minimal read-only NFSv4
server** ("the spike") over the already-vendored go-xdr stubs, serving the `osfs`
backend, and **actually mount it** via `open nfs://localhost:PORT/…` (the
NetFS/automountd path proven feasible in M-004). Confirm the whole pipeline —
server → mount → Finder — end to end. *Then* lift bb-rex's production server (R2)
to replace the spike.

**Why (Architect chose "spike first, then lift").** Now that M-004 shows mounting
works unprivileged here, the highest-value next move is to prove the entire
pipeline end-to-end cheaply: it validates `pkg/virtual` against the *real macOS
NFSv4 client* (not a test harness), surfaces macOS quirks early (R5 territory),
de-risks R1 (timeout) and R4 (mount) together, and produces a tangible mounted
volume. The spike is scaffolding, not the final server — bb-rex remains the
production engine (the "lift, don't write" thesis is intact; the spike is a
de-risking probe, explicitly throwaway).

**Scope of the spike (read-only):** the RPC loop (vendored go-xdr `rpcserver` +
`rpcv2`), NFSv4.0 `COMPOUND` dispatch, and the minimum ops macOS needs to mount
and browse: `PUTROOTFH`, `PUTFH`, `GETFH`, `SAVEFH/RESTOREFH` as needed,
`GETATTR`, `ACCESS`, `LOOKUP`, `READDIR`, `READ`, `READLINK`, plus the minimal
client-state ops macOS demands (`SETCLIENTID`/`SETCLIENTID_CONFIRM` or the 4.1
`EXCHANGE_ID`/`CREATE_SESSION`, discovered empirically from what the client
sends). Reads use the anonymous/special stateid — no `OPEN` state machine. A
file-handle table maps opaque handles ↔ `virtual.Node`.

**Verification (the gate):** `open nfs://localhost:PORT/` mounts; `ls`/`cat` at
the mountpoint return correct data from `osfs`; `mount`/`df` show the volume; the
Architect eyeballs Finder. Then a multi-minute slow read confirms R1 (no
RPC-timeout) for free.

**Roadmap effect:** inserts **R3-spike** before R2. R2 (bb-rex lift) and R3
(production serving) follow, reusing everything the spike teaches about the macOS
client's actual COMPOUND sequence.

---

## DEC-013 — Realize the "spike" AS the bb-rex read-first lift, not a from-scratch server

**Date:** 2026-05-29 · **Status:** accepted (refines DEC-012)

**Decision.** Do not hand-write a from-scratch NFSv4 server for the spike. Instead
**lift bb-rex's server and wire its read path first** — that *is* the spike. Same
intent as DEC-012 (a quick-as-possible, de-risking, read-only-first path to a real
mount), better means.

**Why (realized while sizing the from-scratch build).** macOS won't mount until
the server correctly implements the genuinely hard parts of NFSv4: the FATTR4
attribute-bitmap encoding (`supported_attrs`, `change`, `fsid`, `fileid`, `mode`,
`owner`/`owner_group` as `user@domain` strings, the `time_*` fields,
`mounted_on_fileid`, …), READDIR encoding, and the client-state handshake
(SETCLIENTID/CONFIRM or 4.1 EXCHANGE_ID/CREATE_SESSION/SEQUENCE). These are
exactly what makes bb-rex's `nfs40_program.go` 112 KB — and exactly what a
from-scratch spike would have to reimplement *correctly* before anything mounts.
Hand-writing them is **slower** to a correct mount than lifting the complete,
tested implementation, and it would be throwaway.

**Consequence.** There is no genuinely "fast" path to a macOS NFSv4 mount — it is
intrinsically a multi-session build either way. Given that, the right vehicle is
the one that is also the production server: the bb-rex lift. So the spike collapses
into **R2, sequenced read-path-first**: get LOOKUP/GETATTR/ACCESS/READDIR/READ +
the minimal state handshake mounting against the real macOS client before wiring
the write path. The vendored `rpcserver` (DEC-010 follow-on) and the read-first
sequencing are what remains of the "spike" idea; the from-scratch server is
dropped.

**Next:** R2b — vendor `path`+`filesystem` (DEC-011) — is now the immediate work,
because the lifted server can't compile without them. Then the server files, then
mount read-first.

---

## DEC-014 — R2b executed: `path`+`filesystem` vendored & stripped; the only new dependency (`x/sys/unix`) reimplemented inline

**Date:** 2026-05-29 · **Status:** accepted (executes DEC-011's vendor arm)

**Decision.** DEC-011's vendor step is done. `bb-storage/pkg/{filesystem,
filesystem/path}` are copied into `internal/bb/filesystem/{,path}` (copy +
import-rewrite, the DEC-010 mechanism), the gRPC-status error idiom stripped to
stdlib, and the packages build/vet/test/fmt green standalone. Full provenance and
the strip recipe are in `internal/bb/VENDOR.md`. Two judgment calls beyond the
mechanical recipe, recorded here because a future session would otherwise have to
reverse-engineer them:

1. **`x/sys/unix` reimplemented inline (the one real decision).** `path` stripped
   to pure stdlib exactly as the STATUS recipe predicted (its only external
   coupling was `util`/`grpc-status`, all in error returns). But `filesystem`
   carried a coupling the recipe had *not* flagged: `device_number_unix.go`
   imported `golang.org/x/sys/unix` for dev_t packing (`Mkdev`/`Major`/`Minor`) —
   which would have been Galatea's only non-stdlib runtime dependency, breaking
   AC7 purity. Resolved by inlining the macOS/BSD dev_t encoding in a single
   unconstrained `device_number.go` (Galatea is macOS-only), dropping the
   platform-split `device_number_{unix,nonunix}.go`. The raw encoding is
   immaterial in practice (NFSv4 sends device numbers as a major/minor `specdata4`
   pair, never the raw dev_t; Galatea's backends serve no device nodes), but the
   encoding is macOS-correct. *This is the kind of "measured floor missed one
   transitive import" surprise the loop's verify step exists to catch — `go build`
   flagged it immediately.*

2. **`filesystem/directory.go` kept whole, not trimmed.** It carries bb-storage's
   full local-disk `Directory` interface (`Mount`/`Clonefile`/`OpenAppend`/…),
   which Galatea does not use — but it is also where `DeterministicFileModification`
   `Timestamp` lives, it compiles standalone (interface defs need no impls), and it
   pulls nothing heavy. Trimming it to just the constant is deferrable cleanup, not
   worth the risk of removing a type the server turns out to reference; revisit if
   the dead surface ever bothers us. Same for `file.go`'s unused `File*` interfaces.

**Verification.** `go build/vet/test ./...` green, `go fmt ./...` clean. New smoke
test `internal/bb/filesystem/path/smoke_test.go` resolves a happy path and asserts
the two stripped error paths (null byte; absolute-via-relative-walker) still return
non-nil — guarding the one thing the lift actually changed.

**Not yet wired.** R2b only makes the vendored packages *exist and compile*.
Nothing imports them yet — that is R2c (re-point `pkg/virtual`'s leaf types to
them and fix `pkg/osfs` / the in-memory FSAL / `cmd/galatea`), the next increment.

**What would change this.** If R2c reveals `pkg/virtual` needs a `filesystem` or
`path` symbol that was in a dropped file (none expected — the needed set is
`FileType*`, `DeviceNumber`, `RegionType`, `FileInfo`, `Component`, `Parser`, all
present), re-copy that file. If the dead `Directory`/`File*` interface surface
proves a maintenance burden, trim per call #2 above.
