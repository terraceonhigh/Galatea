# Vendored: libfuse 2.9 headers

These headers — `fuse.h`, `fuse_common.h`, `fuse_opt.h` — are copied verbatim from
upstream **libfuse 2.9** (`FUSE_MAJOR_VERSION 2`, `FUSE_MINOR_VERSION 9`), the
classic macFUSE-compatible ABI. Source: the libfuse tree under `references/`.

**Why vendored:** the libfuse shim (GOAL B / R9, see `docs/GOAL-B-libfuse.md`) is a
drop-in `libfuse.dylib` whose cgo preamble must match the *exact* `struct
fuse_operations` / `fuse_fill_dir_t` an application compiled against. Vendoring the
real headers — rather than transcribing the struct into Go — is the only safe way
to get the layout byte-for-byte (MISTAKES-style "one wrong offset → silent garbage"
avoidance). `struct stat` / `struct fuse_file_info` still come from the macOS SDK
via these headers' own `#include <sys/stat.h>` etc., so they match the app's SDK.

**License:** libfuse is **LGPL-2.1** (`LICENSE` here is its `COPYING.LIB`). LGPL-2.1
is compatible with GPL-3.0-or-later, so these headers may be incorporated into
Galatea (GPLv3+). Only *upstream* libfuse is used; FUSE-T's proprietary NFS bridge
is not (and is not in `references/`). The NFS-backed translation is Galatea's own
work.

**Build flags:** the shim compiles these with `-D_FILE_OFFSET_BITS=64
-DFUSE_USE_VERSION=26` (see the `#cgo CFLAGS` in `../libfuse.go`). Without
`_FILE_OFFSET_BITS=64`, fuse.h errors out / falls back to a `_compat2` struct.
`FUSE_USE_VERSION=26` matches what `hello.c` and most macFUSE-era apps compile to.
