// Package main builds Galatea's drop-in libfuse-compatible shim
// (libfuse 2.9 / macFUSE ABI) as a C-shared library: an application linked
// against -lfuse calls fuse_main(...) and, instead of talking to a kernel FUSE
// driver, gets its operations serviced by Galatea's userspace NFSv4 server.
//
// This is GOAL B / R9 (see docs/GOAL-B-libfuse.md). It is built with
//
//	go build -buildmode=c-shared -o libgalateafuse.dylib ./shim/libfuse
//
// The package is cgo-only; a !cgo stub (stub.go) keeps `CGO_ENABLED=0 go build
// ./...` green (the AC7 CGO-free receipt covers the core, not this edge).
//
// SPIKE STATUS: Phase 0 — de-risking the cgo callback mechanism in three gates
// before any server wiring (see docs/GOAL-B-libfuse.md). The galatea_libfuse_probe_*
// exports and the probe_ops/spike machinery are scaffolding; they are replaced by
// fuse_main_real in Phase 1.
package main

/*
#cgo CFLAGS: -I${SRCDIR}/spike
#include <stdlib.h>
#include <stdint.h>
#include "fuse_probe.h"

// 0b — Go calls the app's getattr through a trampoline (cgo cannot invoke a
// runtime-supplied function pointer directly; a tiny C shim does it).
static int call_probe_getattr(probe_getattr_fn fn, const char *path, char *out, int outlen) {
	return fn(path, out, outlen);
}

// 0c — call_probe_readdir hands the app our filler (the exported Go function
// goProbeFill) plus a uintptr_t handle. It is defined in bridge.c, which
// includes cgo's generated _cgo_export.h to get goProbeFill's exact signature
// (referencing an exported Go symbol from the preamble conflicts with cgo's own
// declaration). Here we only prototype it so Go can call C.call_probe_readdir.
int call_probe_readdir(probe_readdir_fn fn, const char *path, uintptr_t buf);
*/
import "C"

import (
	"fmt"
	"runtime/cgo"
	"unsafe"
)

// galatea_libfuse_smoke — Phase 0a toolchain probe. Removed in Phase 1.
//
//export galatea_libfuse_smoke
func galatea_libfuse_smoke() C.int {
	return 42
}

// galatea_libfuse_probe_getattr — Phase 0b: call the app's getattr funcptr and
// read what C wrote into our buffer. Proves Go -> C-funcptr with in/out params.
//
//export galatea_libfuse_probe_getattr
func galatea_libfuse_probe_getattr(ops *C.struct_probe_ops) C.int {
	cpath := C.CString("/hello")
	defer C.free(unsafe.Pointer(cpath))
	out := make([]byte, 64)
	r := C.call_probe_getattr(ops.getattr, cpath, (*C.char)(unsafe.Pointer(&out[0])), C.int(len(out)))
	got := C.GoString((*C.char)(unsafe.Pointer(&out[0])))
	fmt.Printf("Go: probe getattr -> %d, out=%q\n", int(r), got)
	return r
}

// dirCollector gathers entries the app reports via the filler.
type dirCollector struct{ names []string }

// galatea_libfuse_probe_readdir — Phase 0c: hand the app our filler + a handle
// to a Go collector, let it report entries, then read them back. Proves the
// bidirectional opaque-handle pattern (landmine #1: cgo.Handle, not a Go pointer).
//
//export galatea_libfuse_probe_readdir
func galatea_libfuse_probe_readdir(ops *C.struct_probe_ops) C.int {
	coll := &dirCollector{}
	h := cgo.NewHandle(coll)
	defer h.Delete()
	cpath := C.CString("/")
	defer C.free(unsafe.Pointer(cpath))
	r := C.call_probe_readdir(ops.readdir, cpath, C.uintptr_t(h))
	fmt.Printf("Go: probe readdir -> %d, entries=%v\n", int(r), coll.names)
	return r
}

// goProbeFill is the C-callable filler the app invokes to add a directory entry.
// buf carries a cgo.Handle (passed as uintptr, arriving here as a void*); we
// recover it via uintptr(buf) — the safe, vet-clean direction.
//
//export goProbeFill
func goProbeFill(buf unsafe.Pointer, name *C.char) C.int {
	coll := cgo.Handle(uintptr(buf)).Value().(*dirCollector)
	coll.names = append(coll.names, C.GoString(name))
	return 0
}

func main() {}
