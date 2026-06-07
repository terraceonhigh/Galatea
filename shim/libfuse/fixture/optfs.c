/* optfs.c — a real-tool-shaped FUSE filesystem exercising the option-parsing
 * ABI a tool like sshfs/rclone needs beyond plain fuse_main:
 *
 *   - fuse_opt_parse()  to consume a custom "-o root=DIR" option,
 *   - fuse_main(..., &state)  passing private state as user_data,
 *   - fuse_get_context()->private_data  to retrieve that state inside each op.
 *
 * It is a read-only passthrough to the dir given by -o root=DIR. Proves Galatea's
 * shim provides the fuse_opt_* family + fuse_get_context + fuse_version, not just
 * fuse_main_real — the gate the marquee was actually blocked on.
 *
 *   build: cc -D_FILE_OFFSET_BITS=64 -DFUSE_USE_VERSION=26 -I<shim>/include \
 *             fixture/optfs.c -o optfs -L. -lgalateafuse
 *   run:   optfs -o root=<dir> <mountpoint>
 */
#include "fuse.h"
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <stddef.h>
#include <errno.h>
#include <unistd.h>
#include <fcntl.h>
#include <dirent.h>

struct state { char root[1024]; };

/* fuse_opt spec: capture the value of -o root=... into opts.root. */
struct opts { char *root; };
static struct fuse_opt opt_spec[] = {
	{ "root=%s", offsetof(struct opts, root), 0 },
	FUSE_OPT_END,
};

static const char *rootdir(void) { return ((struct state *)fuse_get_context()->private_data)->root; }
static void full(const char *path, char *out, size_t n) { snprintf(out, n, "%s%s", rootdir(), path); }

static int o_getattr(const char *path, struct stat *st) { char p[2048]; full(path, p, sizeof(p)); return lstat(p, st) == 0 ? 0 : -errno; }
static int o_readdir(const char *path, void *buf, fuse_fill_dir_t filler, off_t off, struct fuse_file_info *fi) {
	char p[2048]; full(path, p, sizeof(p));
	DIR *d = opendir(p); if (!d) return -errno;
	struct dirent *e; while ((e = readdir(d)) != NULL) filler(buf, e->d_name, NULL, 0);
	closedir(d); return 0;
}
static int o_open(const char *path, struct fuse_file_info *fi) { char p[2048]; full(path, p, sizeof(p)); int fd = open(p, O_RDONLY); if (fd < 0) return -errno; close(fd); return 0; }
static int o_read(const char *path, char *buf, size_t size, off_t off, struct fuse_file_info *fi) {
	char p[2048]; full(path, p, sizeof(p));
	int fd = open(p, O_RDONLY); if (fd < 0) return -errno;
	ssize_t n = pread(fd, buf, size, off); close(fd);
	return n < 0 ? -errno : (int)n;
}

static struct fuse_operations o_ops = {
	.getattr = o_getattr, .readdir = o_readdir, .open = o_open, .read = o_read,
};

int main(int argc, char *argv[]) {
	struct fuse_args args = FUSE_ARGS_INIT(argc, argv);
	struct opts o; memset(&o, 0, sizeof(o));
	if (fuse_opt_parse(&args, &o, opt_spec, NULL) == -1) {
		fprintf(stderr, "optfs: option parse failed\n");
		return 1;
	}
	if (!o.root) {
		fprintf(stderr, "usage: %s -o root=<dir> <mountpoint>\n", argv[0]);
		return 2;
	}
	static struct state st;
	strncpy(st.root, o.root, sizeof(st.root) - 1);
	fprintf(stderr, "optfs: fuse_version=%d, root=%s (parsed via fuse_opt_parse)\n", fuse_version(), st.root);
	int r = fuse_main(args.argc, args.argv, &o_ops, &st);
	fuse_opt_free_args(&args);
	return r;
}
