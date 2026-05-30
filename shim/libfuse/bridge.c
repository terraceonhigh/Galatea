/* C bridge for the libfuse shim: the readdir trampoline, which must hand the
 * app's readdir our exported Go filler (goFill). Defined here (not in the cgo
 * preamble) so we can include cgo's generated _cgo_export.h — it declares goFill
 * with the exact signature cgo produced; re-declaring it in the preamble
 * conflicts. */
/* FUSE_USE_VERSION and _FILE_OFFSET_BITS come from the package #cgo CFLAGS. */
#include "fuse.h"
#include <stdint.h>
#include <string.h>
#include <errno.h>
#include "_cgo_export.h"

/* Invoke the app's readdir, handing it our Go filler and the collector handle
 * (carried as uintptr_t, recast to the void* the filler expects). The explicit
 * (fuse_fill_dir_t) cast reconciles cgo's char* against fuse.h's const char*. */
int call_readdir(const struct fuse_operations *op, const char *path, uintptr_t buf) {
	if (!op->readdir) return -ENOSYS;
	struct fuse_file_info fi;
	memset(&fi, 0, sizeof(fi));
	return op->readdir(path, (void *)buf, (fuse_fill_dir_t)goFill, 0, &fi);
}
