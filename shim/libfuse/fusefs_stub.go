package main

// Stub fuse_operations tables for the fuseFS translation tests. They live in a
// normal package file (not _test.go) because Go forbids cgo in test files; the
// stub C functions are `static`, so no symbols leak into the shipped dylib.
//
//   - make_hello_ops:       a fixed read-only /hello tree (Phase 1a read test).
//   - make_passthrough_ops: a real read-WRITE passthrough rooted at pt_root, set
//     via pt_set_root, used for the Phase 2a write test against a host temp dir.
//     It has no .create, so it exercises the mknod()+open() creation fallback.

/*
#include "fuse.h"
#include <stdlib.h>
#include <string.h>
#include <errno.h>
#include <unistd.h>
#include <fcntl.h>
#include <dirent.h>
#include <stdio.h>

// --- fixed read-only hello tree (1a) ---
static int stub_getattr(const char *path, struct stat *st) {
	memset(st, 0, sizeof(*st));
	if (strcmp(path, "/") == 0) { st->st_mode = S_IFDIR | 0755; st->st_nlink = 2; return 0; }
	if (strcmp(path, "/hello") == 0) { st->st_mode = S_IFREG | 0444; st->st_nlink = 1; st->st_size = 13; return 0; }
	return -ENOENT;
}
static int stub_readdir(const char *path, void *buf, fuse_fill_dir_t filler, off_t off, struct fuse_file_info *fi) {
	if (strcmp(path, "/") != 0) return -ENOENT;
	filler(buf, ".", NULL, 0);
	filler(buf, "..", NULL, 0);
	filler(buf, "hello", NULL, 0);
	return 0;
}
static int stub_open(const char *path, struct fuse_file_info *fi) {
	return strcmp(path, "/hello") == 0 ? 0 : -ENOENT;
}
static int stub_read(const char *path, char *buf, size_t size, off_t off, struct fuse_file_info *fi) {
	static const char *s = "Hello World!\n";
	size_t len = 13;
	if (strcmp(path, "/hello") != 0) return -ENOENT;
	if ((size_t)off >= len) return 0;
	if ((size_t)off + size > len) size = len - (size_t)off;
	memcpy(buf, s + (size_t)off, size);
	return (int)size;
}
static struct fuse_operations hello_ops;
static struct fuse_operations *make_hello_ops(void) {
	memset(&hello_ops, 0, sizeof(hello_ops));
	hello_ops.getattr = stub_getattr;
	hello_ops.readdir = stub_readdir;
	hello_ops.open = stub_open;
	hello_ops.read = stub_read;
	return &hello_ops;
}

// --- read-write passthrough rooted at pt_root (2a) ---
static char pt_root[1024];
static void pt_set_root(const char *r) { strncpy(pt_root, r, sizeof(pt_root) - 1); pt_root[sizeof(pt_root)-1] = 0; }
static void pt_full(const char *path, char *out, size_t n) { snprintf(out, n, "%s%s", pt_root, path); }

static int pt_getattr(const char *path, struct stat *st) {
	char p[2048]; pt_full(path, p, sizeof(p));
	return lstat(p, st) == 0 ? 0 : -errno;
}
static int pt_readdir(const char *path, void *buf, fuse_fill_dir_t filler, off_t off, struct fuse_file_info *fi) {
	char p[2048]; pt_full(path, p, sizeof(p));
	DIR *d = opendir(p); if (!d) return -errno;
	struct dirent *e; while ((e = readdir(d)) != NULL) filler(buf, e->d_name, NULL, 0);
	closedir(d); return 0;
}
static int pt_open(const char *path, struct fuse_file_info *fi) {
	char p[2048]; pt_full(path, p, sizeof(p));
	int fd = open(p, O_RDONLY); if (fd < 0) return -errno; close(fd); return 0;
}
static int pt_read(const char *path, char *buf, size_t size, off_t off, struct fuse_file_info *fi) {
	char p[2048]; pt_full(path, p, sizeof(p));
	int fd = open(p, O_RDONLY); if (fd < 0) return -errno;
	ssize_t n = pread(fd, buf, size, off); close(fd);
	return n < 0 ? -errno : (int)n;
}
static int pt_write(const char *path, const char *buf, size_t size, off_t off, struct fuse_file_info *fi) {
	char p[2048]; pt_full(path, p, sizeof(p));
	int fd = open(p, O_WRONLY); if (fd < 0) return -errno;
	ssize_t n = pwrite(fd, buf, size, off); close(fd);
	return n < 0 ? -errno : (int)n;
}
static int pt_mknod(const char *path, mode_t mode, dev_t rdev) {
	char p[2048]; pt_full(path, p, sizeof(p));
	int fd = open(p, O_CREAT | O_EXCL | O_WRONLY, mode); if (fd < 0) return -errno; close(fd); return 0;
}
static int pt_mkdir(const char *path, mode_t mode) { char p[2048]; pt_full(path, p, sizeof(p)); return mkdir(p, mode) == 0 ? 0 : -errno; }
static int pt_unlink(const char *path) { char p[2048]; pt_full(path, p, sizeof(p)); return unlink(p) == 0 ? 0 : -errno; }
static int pt_rmdir(const char *path) { char p[2048]; pt_full(path, p, sizeof(p)); return rmdir(p) == 0 ? 0 : -errno; }
static int pt_rename(const char *from, const char *to) {
	char pf[2048], pt[2048]; pt_full(from, pf, sizeof(pf)); pt_full(to, pt, sizeof(pt));
	return rename(pf, pt) == 0 ? 0 : -errno;
}
static int pt_truncate(const char *path, off_t size) { char p[2048]; pt_full(path, p, sizeof(p)); return truncate(p, size) == 0 ? 0 : -errno; }
static int pt_chmod(const char *path, mode_t mode) { char p[2048]; pt_full(path, p, sizeof(p)); return chmod(p, mode) == 0 ? 0 : -errno; }

// --- A1 structural ops: symlink / readlink / link (real OS calls on pt_root) ---
static int pt_symlink(const char *target, const char *path) {
	// target is stored verbatim (it may be relative to the link); only the link
	// path is rooted under pt_root.
	char p[2048]; pt_full(path, p, sizeof(p));
	return symlink(target, p) == 0 ? 0 : -errno;
}
static int pt_readlink(const char *path, char *buf, size_t size) {
	char p[2048]; pt_full(path, p, sizeof(p));
	ssize_t n = readlink(p, buf, size - 1);
	if (n < 0) return -errno;
	buf[n] = 0; // libfuse contract: NUL-terminate, return 0 (not the length)
	return 0;
}
static int pt_link(const char *from, const char *to) {
	char pf[2048], pt[2048]; pt_full(from, pf, sizeof(pf)); pt_full(to, pt, sizeof(pt));
	return link(pf, pt) == 0 ? 0 : -errno;
}

static struct fuse_operations pt_ops;
static struct fuse_operations *make_passthrough_ops(void) {
	memset(&pt_ops, 0, sizeof(pt_ops));
	pt_ops.getattr = pt_getattr; pt_ops.readdir = pt_readdir; pt_ops.open = pt_open; pt_ops.read = pt_read;
	pt_ops.write = pt_write; pt_ops.mknod = pt_mknod; pt_ops.mkdir = pt_mkdir; pt_ops.unlink = pt_unlink;
	pt_ops.rmdir = pt_rmdir; pt_ops.rename = pt_rename; pt_ops.truncate = pt_truncate; pt_ops.chmod = pt_chmod;
	pt_ops.symlink = pt_symlink; pt_ops.readlink = pt_readlink; pt_ops.link = pt_link;
	return &pt_ops;
}
*/
import "C"

import "unsafe"

// stubHelloOps returns a fuse_operations table modeling hello.c's tree (1a).
func stubHelloOps() unsafe.Pointer {
	return unsafe.Pointer(C.make_hello_ops())
}

// passthroughOps returns a read-write passthrough rooted at the host dir `root`
// (2a). No .create, so it exercises the mknod()+open() creation path.
func passthroughOps(root string) unsafe.Pointer {
	cr := C.CString(root)
	defer C.free(unsafe.Pointer(cr))
	C.pt_set_root(cr)
	return unsafe.Pointer(C.make_passthrough_ops())
}
