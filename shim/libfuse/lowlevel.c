/* lowlevel.c — the libfuse 2.x low-level setup API, as a façade over Galatea's
 * NFS-serving machinery. Real FUSE tools (sshfs, ntfs-3g) don't call the
 * high-level fuse_main(); their main() runs the explicit sequence
 *
 *   fuse_parse_cmdline → fuse_mount → fuse_new → fuse_set_signal_handlers(
 *   fuse_get_session(fuse)) → fuse_daemonize → fuse_loop[_mt] → teardown
 *
 * so the shim must export those symbols. The opaque struct fuse / fuse_chan /
 * fuse_session (declared in the vendored headers) are defined here; they just
 * carry state forward to fuse_loop, where the actual listen + mount_nfs + block
 * happens (we can't mount until ops are bound at fuse_new, and can't bind until
 * the app builds them). fuse_loop forwards to Go (galateaFuseServe), which runs
 * the same serve body as fuse_main_real.
 *
 * STUB PHASE: most bodies are trivial; fuse_loop returns 0 immediately. This
 * exists first to resolve the *link-time symbol surface* (which libfuse symbols
 * a real tool references — including any macOS-specific ones) before the serve
 * loop is wired. See GOAL-B-libfuse.md.
 */
#define FUSE_USE_VERSION 26
#include "fuse.h"
#include <stdlib.h>
#include <string.h>
#include "_cgo_export.h" // galateaFuseServe (the Go serve body)

struct fuse_chan {
	char *mountpoint;
};
struct fuse {
	struct fuse_chan *ch;
	const struct fuse_operations *op;
	size_t op_size;
	void *user_data;
};
struct fuse_session {
	struct fuse *f;
};

// fuse_parse_cmdline extracts the mountpoint (last non-option argv) and the
// threading/foreground flags. The mountpoint must be heap-allocated — the caller
// (sshfs) frees it, so a Go-owned or static string would crash.
int fuse_parse_cmdline(struct fuse_args *args, char **mountpoint,
		int *multithreaded, int *foreground) {
	char *mp = NULL;
	for (int i = 1; i < args->argc; i++) {
		if (args->argv[i] && args->argv[i][0] != '-') {
			mp = args->argv[i];
		}
	}
	if (mountpoint) *mountpoint = mp ? strdup(mp) : NULL;
	if (multithreaded) *multithreaded = 1;
	if (foreground) *foreground = 1;
	return 0;
}

struct fuse_chan *fuse_mount(const char *mountpoint, struct fuse_args *args) {
	(void)args;
	struct fuse_chan *ch = (struct fuse_chan *)calloc(1, sizeof(*ch));
	if (!ch) return NULL;
	ch->mountpoint = mountpoint ? strdup(mountpoint) : NULL;
	return ch;
}

void fuse_unmount(const char *mountpoint, struct fuse_chan *ch) {
	(void)mountpoint;
	if (ch) {
		free(ch->mountpoint);
		free(ch);
	}
}

struct fuse *fuse_new(struct fuse_chan *ch, struct fuse_args *args,
		const struct fuse_operations *op, size_t op_size, void *user_data) {
	(void)args;
	struct fuse *f = (struct fuse *)calloc(1, sizeof(*f));
	if (!f) return NULL;
	f->ch = ch;
	f->op = op;
	f->op_size = op_size;
	f->user_data = user_data;
	return f;
}

void fuse_destroy(struct fuse *f) {
	if (f) free(f);
}

struct fuse_session *fuse_get_session(struct fuse *f) {
	struct fuse_session *se = (struct fuse_session *)calloc(1, sizeof(*se));
	if (!se) return NULL;
	se->f = f;
	return se;
}

int fuse_set_signal_handlers(struct fuse_session *se) {
	(void)se;
	return 0; // the Go serve loop installs its own SIGINT/SIGTERM handling
}
void fuse_remove_signal_handlers(struct fuse_session *se) { (void)se; }

int fuse_daemonize(int foreground) {
	(void)foreground;
	return 0; // always stay foreground; the Go loop owns the process lifetime
}

int fuse_chan_fd(struct fuse_chan *ch) {
	(void)ch;
	return -1;
}

int fuse_is_lib_option(const char *opt) {
	(void)opt;
	return 0;
}

// fuse_loop forwards to Go: listen, mount_nfs, and block until signal — the same
// serve body fuse_main_real runs. The mountpoint, op table, and user_data were
// carried here through fuse_mount/fuse_new. fuse_loop_mt maps to the same call:
// Galatea serves NFS concurrently regardless, and the op calls are serialized by
// the shim's connection mutex (which also guards the single fuse_get_context), so
// there is no separate multithreaded event loop to run.
static int galatea_serve(struct fuse *f) {
	if (!f || !f->ch) return -1;
	return galateaFuseServe(f->ch->mountpoint, (void *)f->op, f->user_data);
}
int fuse_loop(struct fuse *f) { return galatea_serve(f); }
int fuse_loop_mt(struct fuse *f) { return galatea_serve(f); }
