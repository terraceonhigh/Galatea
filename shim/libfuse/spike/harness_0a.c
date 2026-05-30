/* Phase 0a harness: link Galatea's c-shared lib and call one exported Go
 * symbol from a C host. Proves the toolchain + embedded Go runtime. */
#include <stdio.h>
#include "libgalateafuse.h"

int main(void) {
	int v = galatea_libfuse_smoke();
	printf("galatea_libfuse_smoke() = %d (want 42)\n", v);
	return v == 42 ? 0 : 1;
}
