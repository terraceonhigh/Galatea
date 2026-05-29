# bb-storage coupling map

*The empirical de-coupling surface for Phase 1, measured against the source
rather than estimated. Authoritative input to [`DECISIONS.md`](DECISIONS.md)
DEC-001.*

**Measured:** 2026-05-29 · Daedalus
**Subject:** `buildbarn/bb-remote-execution` @ `ed02b7a` (2026-05-13), go 1.26.3
**Method:** `go list -deps` against the reference clone in `references/`, filtered
to `github.com/buildbarn/bb-storage/`. Reproducible; commands below.

---

## The headline

| Strategy | bb-storage packages pulled |
|---|---|
| Naive — `go list -deps ./.../nfsv4` (lift the package whole) | **33** |
| Sever the CAS FSAL implementations from `virtual` | **16** |
| …also strip the `auth`/`jmespath` authorization subtree | **8** |

The naive number includes `cloud/aws`, `cloud/gcp`, `blobstore`,
`blobstore/buffer`, `blobstore/slicing`, `blockdevice`, `capabilities`, `otel`,
`zstd`, `http/client`, and a dozen `proto/configuration/*` packages — i.e. the
entire content-addressed-storage and cloud backend of a build cluster. Galatea
needs none of it. Every one of those enters *transitively*, through the parent
`virtual` package's CAS FSAL implementations — never through the NFSv4 server
logic directly.

## Layer 1 — what nfsv4 imports directly

The six production files (`nfs40_program.go`, `nfs41_program.go`,
`metrics_program.go`, `minor_version_fallback_program.go`, `opened_files_pool.go`,
`system_authenticator.go`) import exactly **7** bb-storage packages:

```
auth          clock          eviction
filesystem    filesystem/path
jmespath      random
```

None imports `blobstore`, `digest`, or `cloud/*`. The server logic is not
CAS-coupled; the parent package it sits in is.

## Layer 2 — the CAS coupling is file-localized in `virtual`

Files in `pkg/filesystem/virtual/` that import `blobstore`/`digest`:

```
blob_access_cas_file_factory.go              node.go  ← investigate
cas_initial_contents_fetcher.go              pool_backed_file_allocator.go
cas_file_factory.go                          resolvable_digest_handle_allocator.go
resolvable_handle_allocating_cas_file_factory.go
stateless_handle_allocating_cas_file_factory.go
```

The interface files we must expose — `directory.go`, `leaf.go` — are **CAS-free**.
All but one of the CAS-coupled files are transparently named (`cas_*` /
`*_cas_*` / handle-allocator factories) and can be left behind. `node.go` is the
single file whose role isn't obvious from its name; it must be classified
(interface-bearing vs. CAS-implementation) before the split is certain.

## Layer 3 — the `auth`/`jmespath` subtree is one file deep

`auth` and `jmespath` are imported by `system_authenticator.go` **only** — not by
any core program file. Between them they pull the entire residual heavy tail:

```
auth → digest, otel, program, proto/auth,
       proto/configuration/digest, jmespath, proto/configuration/jmespath
```

bb-rex uses JMESPath expressions for request authorization. A localhost,
single-user mount (Comprador's case) does not need that. Replace
`system_authenticator.go` with a trivial AUTH_SYS-accepting localhost
authenticator and the whole tail drops.

## The floor — the real shim target

Closure of the 5 utility deps that remain after both cuts
(`clock`, `random`, `eviction`, `filesystem`, `filesystem/path`):

```
clock                 filesystem            random
eviction              filesystem/path       util
proto/configuration/eviction   proto/configuration/tls
```

**8 packages, all stdlib-shaped:** a monotonic clock, a CSPRNG wrapper, an LRU
eviction set, a path type, an `os`-shaped filesystem abstraction, small
error/util helpers, and two trivial config-proto leaves. This is the layer to
shim or vendor. It is at or below the low end of the 500–2,000 LOC estimate in
the kit shopping list.

## Honest caveats

- These are **dependency floors**, derived from import graphs. The split has not
  been executed; the shim LOC is projected, not weighed.
- Go compiles packages whole. The Layer-2 "leave the CAS files behind" move
  requires physically splitting `virtual` into separate packages (interface vs.
  CAS impl), not just not-importing files. That surgery is the actual work.
- `node.go` is the load-bearing unknown. If the interface depends on it and it
  is genuinely CAS-coupled, the clean split costs more than measured.
- Test files were excluded throughout (`internal/mock`, `testutil` are
  test-only). Phase 1's own tests will reintroduce a small test-time surface.

## Reproduce

```sh
cd references/bb-remote-execution
# Layer 0 — naive closure (33)
go list -deps ./pkg/filesystem/virtual/nfsv4 | grep '^github.com/buildbarn/bb-storage/'
# Layer 1 — direct surface (7)
grep -hoE '"github.com/buildbarn/bb-storage/[^"]+"' \
  pkg/filesystem/virtual/nfsv4/*.go --include='*.go' | grep -v _test | sort -u
# Floor — 5 utility deps' closure (8)
go list -deps \
  github.com/buildbarn/bb-storage/pkg/{clock,random,eviction,filesystem,filesystem/path} \
  | grep '^github.com/buildbarn/bb-storage/'
```
