/* passthrough.c — a standalone read-write FUSE filesystem that mirrors a root
 * directory given on the command line. The live gate (Phase 2b) for Galatea's
 * libfuse shim's write path: compiled against libgalateafuse.dylib, it mounts a
 * controlled temp dir read-write through Galatea's NFSv4 server.
 *
 * Unlike libfuse's stock fusexmp (which mirrors the real "/"), this prepends a
 * root prefix to every path, so it only ever touches files under that root —
 * safe to write to. No .create op, so it exercises the shim's mknod()+open()
 * creation fallback. Stateless/path-based (each op opens by path), which is why
 * the shim's no-op VirtualClose and zeroed fuse_file_info are sufficient.
 *
 *   build: cc -D_FILE_OFFSET_BITS=64 -DFUSE_USE_VERSION=26 \
 *             -I<shim>/include fixture/passthrough.c -o ptfs -L. -lgalateafuse
 *   run:   ptfs <rootdir> <mountpoint>
 */
#include "fuse.h"
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <errno.h>
#include <unistd.h>
#include <fcntl.h>
#include <dirent.h>

static char g_root[1024];
static void full(const char *path, char *out, size_t n) { snprintf(out, n, "%s%s", g_root, path); }

static int pt_getattr(const char *path, struct stat *st) {
	char p[2048]; full(path, p, sizeof(p));
	return lstat(p, st) == 0 ? 0 : -errno;
}
static int pt_readdir(const char *path, void *buf, fuse_fill_dir_t filler, off_t off, struct fuse_file_info *fi) {
	char p[2048]; full(path, p, sizeof(p));
	DIR *d = opendir(p); if (!d) return -errno;
	struct dirent *e; while ((e = readdir(d)) != NULL) filler(buf, e->d_name, NULL, 0);
	closedir(d); return 0;
}
static int pt_open(const char *path, struct fuse_file_info *fi) {
	char p[2048]; full(path, p, sizeof(p));
	int fd = open(p, O_RDONLY); if (fd < 0) return -errno; close(fd); return 0;
}
static int pt_read(const char *path, char *buf, size_t size, off_t off, struct fuse_file_info *fi) {
	char p[2048]; full(path, p, sizeof(p));
	int fd = open(p, O_RDONLY); if (fd < 0) return -errno;
	ssize_t n = pread(fd, buf, size, off); close(fd);
	return n < 0 ? -errno : (int)n;
}
static int pt_write(const char *path, const char *buf, size_t size, off_t off, struct fuse_file_info *fi) {
	char p[2048]; full(path, p, sizeof(p));
	int fd = open(p, O_WRONLY); if (fd < 0) return -errno;
	ssize_t n = pwrite(fd, buf, size, off); close(fd);
	return n < 0 ? -errno : (int)n;
}
static int pt_mknod(const char *path, mode_t mode, dev_t rdev) {
	char p[2048]; full(path, p, sizeof(p));
	int fd = open(p, O_CREAT | O_EXCL | O_WRONLY, mode); if (fd < 0) return -errno; close(fd); return 0;
}
static int pt_mkdir(const char *path, mode_t mode) { char p[2048]; full(path, p, sizeof(p)); return mkdir(p, mode) == 0 ? 0 : -errno; }
static int pt_unlink(const char *path) { char p[2048]; full(path, p, sizeof(p)); return unlink(p) == 0 ? 0 : -errno; }
static int pt_rmdir(const char *path) { char p[2048]; full(path, p, sizeof(p)); return rmdir(p) == 0 ? 0 : -errno; }
static int pt_rename(const char *from, const char *to) {
	char pf[2048], pt[2048]; full(from, pf, sizeof(pf)); full(to, pt, sizeof(pt));
	return rename(pf, pt) == 0 ? 0 : -errno;
}
static int pt_truncate(const char *path, off_t size) { char p[2048]; full(path, p, sizeof(p)); return truncate(p, size) == 0 ? 0 : -errno; }
static int pt_chmod(const char *path, mode_t mode) { char p[2048]; full(path, p, sizeof(p)); return chmod(p, mode) == 0 ? 0 : -errno; }

/* A1 structural ops — symlink / readlink / link. The target is stored verbatim
 * (it may be relative to the link); only the link path is rooted under g_root. */
static int pt_symlink(const char *target, const char *path) {
	char p[2048]; full(path, p, sizeof(p));
	return symlink(target, p) == 0 ? 0 : -errno;
}
static int pt_readlink(const char *path, char *buf, size_t size) {
	char p[2048]; full(path, p, sizeof(p));
	ssize_t n = readlink(p, buf, size - 1);
	if (n < 0) return -errno;
	buf[n] = 0; /* libfuse contract: NUL-terminate, return 0 (not the length) */
	return 0;
}
static int pt_link(const char *from, const char *to) {
	char pf[2048], pt[2048]; full(from, pf, sizeof(pf)); full(to, pt, sizeof(pt));
	return link(pf, pt) == 0 ? 0 : -errno;
}

static struct fuse_operations pt_ops = {
	.getattr = pt_getattr, .readdir = pt_readdir, .open = pt_open, .read = pt_read,
	.write = pt_write, .mknod = pt_mknod, .mkdir = pt_mkdir, .unlink = pt_unlink,
	.rmdir = pt_rmdir, .rename = pt_rename, .truncate = pt_truncate, .chmod = pt_chmod,
	.symlink = pt_symlink, .readlink = pt_readlink, .link = pt_link,
};

int main(int argc, char *argv[]) {
	if (argc < 3) {
		fprintf(stderr, "usage: %s <rootdir> <mountpoint>\n", argv[0]);
		return 2;
	}
	strncpy(g_root, argv[1], sizeof(g_root) - 1);
	char *fargv[2] = { argv[0], argv[2] };
	return fuse_main(2, fargv, &pt_ops, NULL);
}
