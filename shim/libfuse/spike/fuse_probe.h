/* Phase 0b/0c spike: a miniature stand-in for struct fuse_operations, to prove
 * the funcptr-callback mechanism before wiring the real libfuse 2.9 surface.
 * Single source of truth for both the cgo preamble and the C harness. */
#pragma once

#include <stddef.h>

/* 0b: an op the Go side calls back through a trampoline (Go -> C funcptr). */
typedef int (*probe_getattr_fn)(const char *path, char *out, int outlen);

/* 0c: a filler the *app* calls to add directory entries (C -> Go funcptr),
 * carrying an opaque buffer token (must be a cgo.Handle, never a Go pointer). */
typedef int (*probe_fill_fn)(void *buf, const char *name);
typedef int (*probe_readdir_fn)(const char *path, void *buf, probe_fill_fn filler);

struct probe_ops {
	probe_getattr_fn getattr;
	probe_readdir_fn readdir;
};
