/* Phase 0b harness: build a struct of function pointers and let the Go side
 * call our getattr back through its trampoline. */
#include <stdio.h>
#include <string.h>
#include "fuse_probe.h"
#include "libgalateafuse.h"

static int my_getattr(const char *path, char *out, int outlen) {
	snprintf(out, outlen, "attr-of(%s)", path);
	return 7;
}

int main(void) {
	struct probe_ops ops = {0};
	ops.getattr = my_getattr;
	int r = galatea_libfuse_probe_getattr(&ops);
	printf("C: probe_getattr returned %d (want 7)\n", r);
	return r == 7 ? 0 : 1;
}
