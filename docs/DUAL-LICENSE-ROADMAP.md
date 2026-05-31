# Dual-license viability — the code roadmap

**Decided 2026-05-30.** Galatea ships **dual-licensed: GPLv3 (the open license,
already in place) + a commercial license** sold to those who ship closed-source
products on it. FOSS developers use it free under GPLv3; a company that profits
from a closed product built on Galatea buys a commercial grant (the "reasonable
chunk"). This out-positions FUSE-T on both flanks: open where FUSE-T is closed,
and a cleaner, standard commercial deal where FUSE-T has a bespoke "commercial
use" clause.

**"Viable" means two things at once:**
1. **Cleanly licensable** — every line in the shipped artifact is *ours to grant*
   under a commercial license (no copyleft code we don't own).
2. **Worth paying for** — a real FUSE-T substitute: the op coverage, lifecycle,
   and polish a macOS developer needs to build on it and ship.

This file is the *code* for both. Non-code viability (a CLA, the commercial
license text, the sales/contact path) is flagged at the end — necessary, but not
code. Phases are dependency-ordered; **Phase L gates everything commercial.**

---

## Phase L — License-clean the base  ⛔ MANDATORY before the first commercial *delivery* — but DEFERRED

**Sequencing (decided 2026-05-30): do the feature phases first; execute Phase L
when a commercial deal is imminent.** Rationale: the vendored **LGPL**
`fuse_opt.c` is *fully compatible inside the GPLv3 open project* — there is no
license problem today, and the open product ships correctly as-is. The LGPL only
blocks the *commercial* grant, which matters only at the moment you hand a
closed-source shipper a dylib. So Phase L gates the first commercial **delivery**,
not feature work and not marketing the dual-license (a `COMMERCIAL.md` "contact
us" can go out anytime). The reimplementation is well-understood and low-risk to
defer; the feature phases below add only our own code (Apache core + ours), so
they don't grow this purge.

When Phase L *is* done: the dylib today links **LGPL** code (`shim/libfuse/
fuse_opt.c`) and compiles against LGPL headers — **you cannot grant a proprietary
license over code you don't own.** Clean-room hygiene: write the replacement from
the documented behavior + our own tests, not by transcribing the LGPL source the
tree has carried (knowing the *shape* is fine; the *expression* must be ours).
Keep the LGPL quarantined and labelled (VENDOR.md already does) until then.

- **L1 — Reimplement `fuse_opt`.** A clean-room option parser
  (`fuse_opt_parse`/`match`/`add_arg`/`insert_arg`/`add_opt`/`free_args`) matching
  libfuse 2.9's *behavior* (the man page + a behavior test suite — **not** the LGPL
  source; keep it clean-room). Replaces `fuse_opt.c`. ~1 wk.
- **L2 — Own the headers.** Replace the vendored LGPL `fuse.h`/`fuse_common.h`/
  `fuse_opt.h` with our own ABI-compatible declarations (the structs + signatures
  we already match byte-for-byte). ~few days. (APIs are likely uncopyrightable
  post-*Google v. Oracle*, but clean headers remove all doubt.)
- **L3 — License hygiene.** SPDX headers on our files; keep `internal/bb` +
  `internal/xdr` Apache-2.0 with a `NOTICE`; `LICENSE` (GPLv3) + `COMMERCIAL.md`
  (commercial-license contact + terms sketch); a CI license scan that fails on any
  LGPL/AGPL/incompatible code in the shipped tree.
- **Gate:** the scan shows only Apache-2.0 (vendored, attributed) + our
  dual-licensed code. The shipped dylib is ours to grant. **Est. ~2 wks.**

## Phase A — Full libfuse-2.x op coverage

Today the shim implements ~18 of ~40 `fuse_operations`. A real tool needs the rest.

- **A1 — Structural ops ✅✅ (shim + LIVE, 2026-05-30):** `symlink`/`readlink`/
  `link` wired into the shim (`VirtualSymlink`→`op->symlink`,
  `VirtualGetAttributes` symlink-target→`op->readlink`, `VirtualLink`→`op->link`),
  green against the passthrough stub (`TestFuseFSLinks` — real host symlink/
  readlink/hard-link + EXDEV guard + target-shape round-trips), race-clean,
  CGO-free build held. Commits `cfcc3f3`, `d110461`.
  **LIVE GATE MET (Architect-run on macOS, commit `74e76af`):** the
  `run-a1-live.sh` harness builds dylib + passthrough fixture, mounts through
  the NFSv4 server (`mount_nfs`, unprivileged), and ran 6/6 A1 checks **over the
  real mount** — `ln -s` + `readlink` + cat-through-link + absolute-target
  round-trip + hard link with `nlink==2` — no kext, no FUSE-T, no root; clean
  teardown. This is the over-the-wire tier (CREATE-NF4LNK→LOOKUP→READLINK→LINK as
  real COMPOUNDs over the NFS client) that R9 1b/2b set as the bar.
- **A1-times — `utimens` (mtime) ✅✅ LIVE-PROVEN (2026-05-30).** The first
  server-layer attribute lift, proven end to end (`run-a1-live.sh`, 9/9). Three
  pieces, all needed:
  1. **Advertise** `FATTR4_TIME_ACCESS_SET`(48) + `FATTR4_TIME_MODIFY_SET`(54) in
     `FATTR4_SUPPORTED_ATTRS` — *the* live-only bug: the macOS client sends a
     SETATTR only for attributes the server advertises, so without this it
     skipped the time-set entirely and `touch` no-opped (commit `98fc979`).
  2. **Decode** `TIME_MODIFY_SET` (the *writable* mtime settime4 — not read-only
     `TIME_MODIFY`/53) and accept+consume `TIME_ACCESS_SET` (`touch` sets atime
     AND mtime together; rejecting it failed the whole SETATTR). atime is
     consumed-not-stored (noatime-style).
  3. **Apply** through the in-memory FSAL (stores/emits mtime) and the shim
     (`VirtualSetAttributes`→`op->utimens`).
  Wire-confirmed by an env-gated `GALATEA_SETATTR_TRACE`: `touch -t` →
  `TIME_ACCESS_SET|TIME_MODIFY_SET`, 32 bytes (two `SET_TO_CLIENT_TIME`) → mtime
  lands; plain `touch` → same mask, 8 bytes (two `SET_TO_SERVER_TIME`, no stamp)
  → decoded but **not applied** (no wall clock — the deferred path, now backed by
  wire evidence). Headless: `TestConformanceSetattrMtime` (macOS-shaped 3-attr,
  distinct atime≠mtime) + `TestFuseFSUtimens`. Commits `6d5a1cd`/`3b4a8e0`/
  `52032d8`/`73eb6c8`/`98fc979`.
- **A1-ceiling — still a server-layer task, not shim wiring** (verified by reading
  the dispatch before wiring — the advisor's "compiles-and-lies" check):
  - `chown` — `fattr4ToAttributes` returns **NFS4ERR_PERM** for `OWNER`/
    `OWNER_GROUP`. The wire form is `user@domain`, so lifting it drags in
    id-mapping (name vs numeric-string, client domain) — a policy decision, not
    mechanics; deferred deliberately.
  - `utimens` **atime** + **`SET_TO_SERVER_TIME`** — atime needs a new field in
    `virtual.Attributes` (mtime-only ships today); server-time needs a wall clock
    the deterministic server lacks. Both deferred; `op->utimens` passes
    `UTIME_OMIT` for atime meanwhile.
  - `fallocate` — **no `OP_ALLOCATE`** in the lifted NFSv4.0 server (it's a 4.2
    op).
  - `statfs` — no `virtual` hook; free-space is FATTR4 space-* through GETATTR.
- **A1-misc — `flush`/`fsync`/`fsyncdir`/`access`:** handled at the
  server/client layer without a distinct FSAL op (ACCESS via GETATTR+mode;
  flush/fsync are client-side or sane no-ops) — no shim work needed.
- **A2 — Extended attributes:** `setxattr`/`getxattr`/`listxattr`/`removexattr` →
  NFSv4 named attributes (today the FSAL reports `HasNamedAttributes=false`; real
  support wires named-attr directories through the server + FSAL). ~1–2 wks, or
  document as an unsupported ceiling.
- **A3 — macFUSE macOS extensions:** `setvolname`, `exchange`, `getxtimes`,
  `setcrtime`/`setbkuptime`, `chflags`, `setattr_x`/`fsetattr_x` — what *Mac-native*
  FUSE apps use. Implement what maps to NFS; document the rest. ~1 wk partial.
- **Gate:** a representative real C FUSE tool (ntfs-3g, or sshfs-2.x) runs its full
  op set through the shim. **Est. ~3–5 wks.**

## Phase S — Full open-state semantics

Today open/read/write is per-op atomic (open→op→release *each call*) — correct for
path-based fs's and cgofuse's memfs, wrong for fs's that hold state across an open
session.

- **S1 — Persistent handle threading:** `open()` once → a stable `fuse_file_info`
  (carrying the fs's `fh`) threaded through every read/write until `release()`,
  keyed off the NFS open-state the server already tracks (OPEN/CLOSE). `fuseConn`
  grows an open-handle table.
- **S2 — flush/fsync/release** in the correct order.
- **Gate:** a stateful fs (relies on `fh` across the session) works, not just
  stateless ones. **Est. ~1–2 wks.**

## Phase M — Mount lifecycle & daemon

Today: `mount_nfs` + signal-unmount, single mount, no eject integration.

- **M1 — `galatea.Mount()`:** NetFS (`NetFSMountURLAsync`) or `mount_nfs` +
  **DiskArbitration** for a real Finder volume (named, ejectable).
- **M2 — Lifecycle:** sleep/wake survival, graceful unmount, **multi-mount**,
  mount options (volname, ro, allow_other, …).
- **M3 — A launchd agent/daemon** that owns mounts — the background "it just works"
  UX FUSE-T ships.
- **Gate:** Finder volume with eject; survives sleep/wake; concurrent mounts.
  **Est. ~3–4 wks.**

## Phase 3 — libfuse-3.x ABI

Many current tools (today's sshfs, newer fs's) are fuse3, not 2.x.

- **3.1 — A second ABI front** (`libfuse.3.dylib`-compatible): fuse3
  `fuse_operations` (3-arg ops, `fuse_config` in `init`, `readdir` flags), fuse3
  `fuse_main`/`fuse_loop`, `fuse_lowlevel` deltas. The `fuseFS` translation core is
  reusable; the C ABI/struct layer doubles.
- **Gate:** a fuse3 tool runs through the shim. **Est. ~2–4 wks.**

## Phase D — Distribution & trust

- **D1 — Sign + notarize:** Developer-ID + `notarytool` (the `macos-notarize`
  skill) → a notarized `.dmg`/`.pkg`. ~days.
- **D2 — Install story:** daemon + dylib placement so tools find Galatea's libfuse
  (generalize the `CGOFUSE_LIBFUSE_PATH`/install-name path); a "use Galatea instead
  of macFUSE/FUSE-T" switch.
- **D3 — Developer docs:** a "port your FUSE app to Galatea" guide; the fixtures
  (`hello`, passthrough, `optfs`) become samples; an API reference.

---

## Minimum-viable vs. full

- **Build order:** the feature phases first (common subset of **A** + **M** +
  **D1**), then **Phase L** executed at the first commercial delivery — it's the
  gate on *selling closed*, not on building or marketing. A macOS dev can link the
  GPL build the whole time; the commercial grant becomes deliverable once L lands.
  fuse3 (Phase 3), full xattrs (A2), and full open-state (S) follow as the market
  asks. **Rough envelope to minimum-sellable: ~6–10 focused weeks** (L's ~2 are
  spent last, when a buyer is real).
- **Full FUSE-T parity:** all phases incl. Phase 3 and the macOS extensions.
  **~3–4 months.**

## Caveats carried forward

- **Two-Go-runtime tax.** The shim is Go, so a *Go* FUSE tool (rclone) co-loads two
  runtimes (a background signal/scheduler tax — memfs is clean, heavy concurrent
  rclone is unproven). *C* tools are unaffected — position the product for C tools
  first. Removing the tax means a C core: a large rewrite, deliberately **not** on
  this roadmap.
- **FSKit is the native endgame.** This NFS-loopback shim is the bridge; the
  durable native target is a FSKit module (the Onfim cathedral framing). Don't
  over-invest in matching every FUSE-T corner if FSKit is where the long-term value
  is. The commercial story: *"the open, license-clean FUSE-T today; FSKit-native
  tomorrow."*

## Non-code prerequisites (necessary for viability)

Not code, but the dual-license isn't *viable* without them:
- A **Contributor License Agreement** before opening contribution — otherwise a
  single GPL-only contribution poisons the ability to sell commercial licenses.
- The **commercial license text** + a contact/sales path (`COMMERCIAL.md`).
- The "reasonable chunk" is a **per-deal negotiation** the dual-license *enables*
  (flat fee / per-seat / revenue share), set when a commercial licensee asks — the
  LICENSE only needs "GPLv3 + for commercial licensing, contact …".
