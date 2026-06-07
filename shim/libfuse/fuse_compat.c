/* fuse_compat.c — the small pieces of the libfuse API a real tool needs beyond
 * fuse_main + the fuse_opt_* family (which is vendored in fuse_opt.c):
 *
 *   fuse_version()      — the compiled API version (2.9 -> 29).
 *   fuse_get_context()  — the calling context; tools read private_data (the
 *                         user_data they passed to fuse_main) and uid/gid here.
 *
 * The context is a static struct, which is safe because the shim serialises all
 * calls into the app's operations (one op at a time), so fuse_get_context is
 * never invoked from two ops concurrently. uid/gid are the server process's for
 * now; per-request AUTH_SYS identity is later work. */
#include "fuse.h"
#include <unistd.h>

static void *g_user_data = NULL;

/* Called by fuse_main_real (bridge.c) with the app's user_data so a later
 * fuse_get_context() can hand it back as private_data. */
void galatea_set_user_data(void *ud) { g_user_data = ud; }

int fuse_version(void) { return FUSE_VERSION; }

static struct fuse_context g_ctx;

struct fuse_context *fuse_get_context(void) {
	g_ctx.fuse = NULL;
	g_ctx.uid = getuid();
	g_ctx.gid = getgid();
	g_ctx.pid = 0;
	g_ctx.private_data = g_user_data;
	g_ctx.umask = 0;
	return &g_ctx;
}
