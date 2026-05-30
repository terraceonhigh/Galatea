package main

// This file provides a stub fuse_operations table (modeling hello.c's tree) for
// the fuseFS translation test. It lives in a normal package file rather than a
// _test.go because Go does not permit cgo (`import "C"`) in test files. The stub
// C functions are `static`, so no symbols leak into the shipped dylib; the only
// cost is a little dead Go code (stubHelloOps) the production build never calls.

/*
#include "fuse.h"
#include <string.h>
#include <errno.h>

static int stub_getattr(const char *path, struct stat *st) {
	memset(st, 0, sizeof(*st));
	if (strcmp(path, "/") == 0) { st->st_mode = S_IFDIR | 0755; st->st_nlink = 2; return 0; }
	if (strcmp(path, "/hello") == 0) { st->st_mode = S_IFREG | 0444; st->st_nlink = 1; st->st_size = 13; return 0; }
	return -ENOENT;
}
static int stub_readdir(const char *path, void *buf, fuse_fill_dir_t filler, off_t off, struct fuse_file_info *fi) {
	if (strcmp(path, "/") != 0) return -ENOENT;
	filler(buf, ".", NULL, 0);     // the . and .. the translation must drop
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
static struct fuse_operations stub_ops;
static struct fuse_operations *make_stub_ops(void) {
	memset(&stub_ops, 0, sizeof(stub_ops));
	stub_ops.getattr = stub_getattr;
	stub_ops.readdir = stub_readdir;
	stub_ops.open = stub_open;
	stub_ops.read = stub_read;
	return &stub_ops;
}
*/
import "C"

import "unsafe"

// stubHelloOps returns a fuse_operations table modeling hello.c's tree.
func stubHelloOps() unsafe.Pointer {
	return unsafe.Pointer(C.make_stub_ops())
}
