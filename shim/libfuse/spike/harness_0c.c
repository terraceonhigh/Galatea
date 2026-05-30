/* Phase 0c harness: implement readdir, which calls the Go-supplied filler to
 * report entries. Proves C -> Go funcptr + the opaque-handle round-trip. */
#include <stdio.h>
#include "fuse_probe.h"
#include "libgalateafuse.h"

static int my_readdir(const char *path, void *buf, probe_fill_fn filler) {
	filler(buf, ".");
	filler(buf, "..");
	filler(buf, "hello");
	return 0;
}

int main(void) {
	struct probe_ops ops = {0};
	ops.readdir = my_readdir;
	int r = galatea_libfuse_probe_readdir(&ops);
	printf("C: probe_readdir returned %d (want 0)\n", r);
	return r == 0 ? 0 : 1;
}
