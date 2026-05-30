/* C bridge for the libfuse shim: trampolines that hand exported Go functions to
 * the app as callbacks. Defined here (not in the cgo preamble) so we can include
 * cgo's generated _cgo_export.h, which declares the exported Go symbols with the
 * exact signatures cgo produced — re-declaring them in the preamble conflicts. */
#include <stdint.h>
#include "fuse_probe.h"
#include "_cgo_export.h"

/* 0c — invoke the app's readdir, handing it our Go filler and the collector
 * handle (carried as a uintptr_t, recast to the void* the filler expects). The
 * explicit (probe_fill_fn) cast reconciles cgo's char* against the typedef's
 * const char* — the same cast Phase 1 will use against fuse_fill_dir_t. */
int call_probe_readdir(probe_readdir_fn fn, const char *path, uintptr_t buf) {
	return fn(path, (void *)buf, (probe_fill_fn)goProbeFill);
}
