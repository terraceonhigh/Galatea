// Package main builds Galatea's drop-in libfuse-compatible shim
// (libfuse 2.9 / macFUSE ABI) as a C-shared library: an application linked
// against -lfuse calls fuse_main(...) and, instead of talking to a kernel FUSE
// driver, gets its operations serviced by Galatea's userspace NFSv4 server.
//
// This is GOAL B / R9 (see docs/GOAL-B-libfuse.md). Built with
//
//	go build -buildmode=c-shared -o libgalateafuse.dylib ./shim/libfuse
//
// The package is cgo-only; a !cgo stub (stub.go) keeps `CGO_ENABLED=0 go build
// ./...` green (the AC7 CGO-free receipt covers the core, not this edge).
//
// SPIKE STATUS: Phase 1a — the fuseFS translation layer, exercised by a Go test
// against a stub C ops table (fusefs_test.go). No NFS, no mount yet (Phase 1b
// adds fuse_main_real + mount_nfs + the live hello.c gate). The cgo callback
// mechanism this rides on was de-risked in Phase 0.
//
// fuseFS mirrors pkg/osfs: it presents a path-keyed node tree over the
// virtual.Directory/Leaf interface, synthesised from the app's path-based FUSE
// operations exactly as osfs synthesises from os.Stat/os.ReadDir. Read paths are
// live; mutating ops return ROFS for the spike.
package main

/*
#cgo CFLAGS: -I${SRCDIR}/include -D_FILE_OFFSET_BITS=64 -DFUSE_USE_VERSION=26
#include "fuse.h"
#include <stdlib.h>
#include <string.h>
#include <errno.h>

// Trampolines: cgo cannot invoke a runtime-supplied function pointer directly,
// so these tiny C shims call the app's ops. All calls are serialised by the
// fuseConn mutex on the Go side (libfuse apps assume single-threaded by default).
static int call_getattr(const struct fuse_operations *op, const char *path, struct stat *st) {
	if (!op->getattr) return -ENOSYS;
	return op->getattr(path, st);
}
static int call_open(const struct fuse_operations *op, const char *path, struct fuse_file_info *fi) {
	if (!op->open) return 0; // open is optional; its absence means "ok to read"
	return op->open(path, fi);
}
static int call_read(const struct fuse_operations *op, const char *path, char *buf, size_t size, off_t off, struct fuse_file_info *fi) {
	if (!op->read) return -ENOSYS;
	return op->read(path, buf, size, off, fi);
}

// call_readdir is defined in bridge.c, which includes _cgo_export.h to reference
// the exported Go filler goFill with cgo's exact signature.
int call_readdir(const struct fuse_operations *op, const char *path, uintptr_t buf);
*/
import "C"

import (
	"context"
	"hash/fnv"
	"io"
	"path"
	"runtime/cgo"
	"sort"
	"strings"
	"sync"
	"unsafe"

	"github.com/terraceonhigh/galatea/pkg/virtual"
)

// dirCollector gathers entry names the app reports via the filler.
type dirCollector struct{ names []string }

// goFill is the C-callable fuse_fill_dir_t the app invokes for each directory
// entry. buf carries a cgo.Handle (passed as uintptr, arriving as void*); we
// recover it via uintptr(buf). stbuf may be NULL (hello passes it so) — we do
// not dereference it; per-child getattr fills attributes later. Return 0 to keep
// the app reporting (1 would mean "buffer full").
//
//export goFill
func goFill(buf unsafe.Pointer, name *C.char, stbuf *C.struct_stat, off C.off_t) C.int {
	coll := cgo.Handle(uintptr(buf)).Value().(*dirCollector)
	coll.names = append(coll.names, C.GoString(name))
	return 0
}

// fuseConn holds the app's operation table and serialises every call into it.
type fuseConn struct {
	op *C.struct_fuse_operations
	mu sync.Mutex
}

// NewFuseRoot builds a Galatea FSAL over an app's fuse_operations table and the
// matching HandleResolver. ops is taken as unsafe.Pointer because a typed
// *C.struct_fuse_operations is not assignable across cgo files (the caller — the
// test, or fuse_main_real — mints its own cgo type for the same C struct).
func NewFuseRoot(ops unsafe.Pointer) (virtual.Directory, virtual.HandleResolver) {
	c := &fuseConn{op: (*C.struct_fuse_operations)(ops)}
	root := &fuseDir{conn: c, path: "/"}
	return root, c.resolveHandle
}

// getattr calls the app's getattr under the lock and maps the result.
func (c *fuseConn) getattr(p string) (C.struct_stat, virtual.Status) {
	cp := C.CString(p)
	defer C.free(unsafe.Pointer(cp))
	var st C.struct_stat
	c.mu.Lock()
	r := C.call_getattr(c.op, cp, &st)
	c.mu.Unlock()
	switch {
	case r == 0:
		return st, virtual.StatusOK
	case r == -C.ENOENT:
		return st, virtual.StatusErrNoEnt
	default:
		return st, virtual.StatusErrIO
	}
}

// readdir calls the app's readdir under the lock, collecting entry names. The
// filler (goFill) runs *inside* this call and must not touch c.mu — it only
// appends to the collector via the handle. "." and ".." are dropped: the
// virtual layer never expects them (osfs via os.ReadDir never sees them), and a
// getattr("/.") would ENOENT and break the listing.
func (c *fuseConn) readdir(p string) ([]string, virtual.Status) {
	coll := &dirCollector{}
	h := cgo.NewHandle(coll)
	defer h.Delete()
	cp := C.CString(p)
	defer C.free(unsafe.Pointer(cp))
	c.mu.Lock()
	r := C.call_readdir(c.op, cp, C.uintptr_t(h))
	c.mu.Unlock()
	if r != 0 {
		return nil, virtual.StatusErrIO
	}
	out := coll.names[:0]
	for _, n := range coll.names {
		if n == "." || n == ".." {
			continue
		}
		out = append(out, n)
	}
	return out, virtual.StatusOK
}

// resolveHandle decodes a path-based file handle (the node's absolute FUSE
// path), verifies it via getattr, and returns the node.
func (c *fuseConn) resolveHandle(r io.ByteReader) (virtual.DirectoryChild, virtual.Status) {
	var sb strings.Builder
	for {
		b, err := r.ReadByte()
		if err == io.EOF {
			break
		}
		if err != nil {
			return virtual.DirectoryChild{}, virtual.StatusErrBadHandle
		}
		sb.WriteByte(b)
	}
	p := sb.String()
	if p == "" || p[0] != '/' {
		return virtual.DirectoryChild{}, virtual.StatusErrBadHandle
	}
	st, status := c.getattr(p)
	if status != virtual.StatusOK {
		return virtual.DirectoryChild{}, virtual.StatusErrStale
	}
	return childOf(c.nodeFor(p, &st)), virtual.StatusOK
}

// nodeFor builds the node (directory or leaf) for a path given its stat.
func (c *fuseConn) nodeFor(p string, st *C.struct_stat) virtual.Node {
	if uint32(st.st_mode)&uint32(C.S_IFMT) == uint32(C.S_IFDIR) {
		return &fuseDir{conn: c, path: p}
	}
	return &fuseFile{conn: c, path: p}
}

// childOf wraps a node as a DirectoryChild.
func childOf(n virtual.Node) virtual.DirectoryChild {
	var ch virtual.DirectoryChild
	if dir, ok := n.(virtual.Directory); ok {
		return ch.FromDirectory(dir)
	}
	return ch.FromLeaf(n.(virtual.Leaf))
}

// pathFileID derives a stable, nonzero fileid from the path — used when the app
// leaves st_ino == 0 (hello does). Handles stay path-based; this only feeds the
// FATTR4 fileid attribute, which a client dislikes seeing as 0.
func pathFileID(p string) uint64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(p))
	v := h.Sum64()
	if v == 0 {
		v = 1
	}
	return v
}

// statToAttrs translates a C struct stat into virtual.Attributes, setting the
// mandatory attributes the NFSv4 server requires (FileHandle,
// HasNamedAttributes, IsInNamedAttributeDirectory — MISTAKES.md M-006).
func statToAttrs(st *C.struct_stat, p string, requested virtual.AttributesMask, a *virtual.Attributes) {
	mode := uint32(st.st_mode)
	ft := virtual.FileTypeRegularFile
	switch mode & uint32(C.S_IFMT) {
	case uint32(C.S_IFDIR):
		ft = virtual.FileTypeDirectory
	case uint32(C.S_IFLNK):
		ft = virtual.FileTypeSymlink
	case uint32(C.S_IFIFO):
		ft = virtual.FileTypeFIFO
	case uint32(C.S_IFSOCK):
		ft = virtual.FileTypeSocket
	case uint32(C.S_IFCHR):
		ft = virtual.FileTypeCharacterDevice
	case uint32(C.S_IFBLK):
		ft = virtual.FileTypeBlockDevice
	case uint32(C.S_IFREG):
		ft = virtual.FileTypeRegularFile
	}
	if requested&virtual.AttributesMaskFileType != 0 {
		a.SetFileType(ft)
	}
	if requested&virtual.AttributesMaskPermissions != 0 {
		a.SetPermissions(virtual.NewPermissionsFromMode(mode & 0o777))
	}
	if requested&virtual.AttributesMaskSizeBytes != 0 {
		size := int64(st.st_size)
		if size < 0 {
			size = 0
		}
		a.SetSizeBytes(uint64(size))
	}
	if requested&virtual.AttributesMaskFileHandle != 0 {
		a.SetFileHandle([]byte(p))
	}
	if requested&virtual.AttributesMaskHasNamedAttributes != 0 {
		a.SetHasNamedAttributes(false)
	}
	if requested&virtual.AttributesMaskIsInNamedAttributeDirectory != 0 {
		a.SetIsInNamedAttributeDirectory(false)
	}
	fileid := uint64(st.st_ino)
	if fileid == 0 {
		fileid = pathFileID(p)
	}
	if requested&virtual.AttributesMaskInodeNumber != 0 {
		a.SetInodeNumber(fileid)
	}
	nlink := uint32(st.st_nlink)
	if nlink == 0 {
		nlink = 1
	}
	if requested&virtual.AttributesMaskLinkCount != 0 {
		a.SetLinkCount(nlink)
	}
	if requested&virtual.AttributesMaskChangeID != 0 {
		a.SetChangeID(fileid)
	}
}

// --- directory ------------------------------------------------------------

type fuseDir struct {
	conn *fuseConn
	path string
}

func (d *fuseDir) VirtualGetAttributes(ctx context.Context, requested virtual.AttributesMask, a *virtual.Attributes) {
	st, status := d.conn.getattr(d.path)
	if status != virtual.StatusOK {
		if requested&virtual.AttributesMaskFileType != 0 {
			a.SetFileType(virtual.FileTypeDirectory)
		}
		return
	}
	statToAttrs(&st, d.path, requested, a)
}

func (d *fuseDir) VirtualLookup(ctx context.Context, name virtual.Component, requested virtual.AttributesMask, out *virtual.Attributes) (virtual.DirectoryChild, virtual.Status) {
	childPath := path.Join(d.path, name.String())
	st, status := d.conn.getattr(childPath)
	if status != virtual.StatusOK {
		return virtual.DirectoryChild{}, status
	}
	statToAttrs(&st, childPath, requested, out)
	return childOf(d.conn.nodeFor(childPath, &st)), virtual.StatusOK
}

func (d *fuseDir) VirtualReadDir(ctx context.Context, firstCookie uint64, requested virtual.AttributesMask, reporter virtual.DirectoryEntryReporter) virtual.Status {
	names, status := d.conn.readdir(d.path)
	if status != virtual.StatusOK {
		return status
	}
	sort.Strings(names)
	for i, n := range names {
		cookie := uint64(i + 1)
		if cookie <= firstCookie {
			continue
		}
		comp, ok := virtual.NewComponent(n)
		if !ok {
			continue
		}
		childPath := path.Join(d.path, n)
		st, gst := d.conn.getattr(childPath)
		if gst != virtual.StatusOK {
			continue
		}
		var a virtual.Attributes
		statToAttrs(&st, childPath, requested, &a)
		if !reporter.ReportEntry(cookie, comp, childOf(d.conn.nodeFor(childPath, &st)), &a) {
			break
		}
	}
	return virtual.StatusOK
}

func (d *fuseDir) VirtualOpenChild(ctx context.Context, name virtual.Component, shareAccess virtual.ShareMask, createAttributes *virtual.Attributes, existingOptions *virtual.OpenExistingOptions, requested virtual.AttributesMask, openedFileAttributes *virtual.Attributes) (virtual.Leaf, virtual.AttributesMask, virtual.ChangeInfo, virtual.Status) {
	ci := virtual.ChangeInfo{}
	childPath := path.Join(d.path, name.String())
	st, status := d.conn.getattr(childPath)
	if status == virtual.StatusErrNoEnt {
		if createAttributes != nil {
			return nil, 0, ci, virtual.StatusErrROFS
		}
		return nil, 0, ci, virtual.StatusErrNoEnt
	}
	if status != virtual.StatusOK {
		return nil, 0, ci, status
	}
	if existingOptions == nil {
		return nil, 0, ci, virtual.StatusErrExist
	}
	if uint32(st.st_mode)&uint32(C.S_IFMT) == uint32(C.S_IFDIR) {
		return nil, 0, ci, virtual.StatusErrIsDir
	}
	if shareAccess&virtual.ShareMaskWrite != 0 {
		return nil, 0, ci, virtual.StatusErrROFS
	}
	leaf := &fuseFile{conn: d.conn, path: childPath}
	statToAttrs(&st, childPath, requested, openedFileAttributes)
	return leaf, 0, ci, virtual.StatusOK
}

func (d *fuseDir) VirtualApply(data any) bool { return false }

func (d *fuseDir) VirtualSetAttributes(ctx context.Context, in *virtual.Attributes, requested virtual.AttributesMask, a *virtual.Attributes) virtual.Status {
	return virtual.StatusErrROFS
}
func (d *fuseDir) VirtualOpenNamedAttributes(ctx context.Context, createDirectory bool, requested virtual.AttributesMask, a *virtual.Attributes) (virtual.Directory, virtual.Status) {
	return nil, virtual.StatusErrNoEnt
}
func (d *fuseDir) VirtualLink(ctx context.Context, name virtual.Component, leaf virtual.Leaf, requested virtual.AttributesMask, a *virtual.Attributes) (virtual.ChangeInfo, virtual.Status) {
	return virtual.ChangeInfo{}, virtual.StatusErrROFS
}
func (d *fuseDir) VirtualMkdir(name virtual.Component, requested virtual.AttributesMask, a *virtual.Attributes) (virtual.Directory, virtual.ChangeInfo, virtual.Status) {
	return nil, virtual.ChangeInfo{}, virtual.StatusErrROFS
}
func (d *fuseDir) VirtualMknod(ctx context.Context, name virtual.Component, fileType virtual.FileType, requested virtual.AttributesMask, a *virtual.Attributes) (virtual.Leaf, virtual.ChangeInfo, virtual.Status) {
	return nil, virtual.ChangeInfo{}, virtual.StatusErrROFS
}
func (d *fuseDir) VirtualRename(oldName virtual.Component, newDirectory virtual.Directory, newName virtual.Component) (virtual.ChangeInfo, virtual.ChangeInfo, virtual.Status) {
	return virtual.ChangeInfo{}, virtual.ChangeInfo{}, virtual.StatusErrROFS
}
func (d *fuseDir) VirtualRemove(name virtual.Component, removeDirectory, removeLeaf bool) (virtual.ChangeInfo, virtual.Status) {
	return virtual.ChangeInfo{}, virtual.StatusErrROFS
}
func (d *fuseDir) VirtualSymlink(ctx context.Context, pointedTo virtual.Parser, linkName virtual.Component, requested virtual.AttributesMask, a *virtual.Attributes) (virtual.Leaf, virtual.ChangeInfo, virtual.Status) {
	return nil, virtual.ChangeInfo{}, virtual.StatusErrROFS
}

// --- file -----------------------------------------------------------------

type fuseFile struct {
	conn *fuseConn
	path string
}

func (f *fuseFile) VirtualGetAttributes(ctx context.Context, requested virtual.AttributesMask, a *virtual.Attributes) {
	st, status := f.conn.getattr(f.path)
	if status != virtual.StatusOK {
		if requested&virtual.AttributesMaskFileType != 0 {
			a.SetFileType(virtual.FileTypeRegularFile)
		}
		return
	}
	statToAttrs(&st, f.path, requested, a)
}

func (f *fuseFile) VirtualOpenSelf(ctx context.Context, shareAccess virtual.ShareMask, options *virtual.OpenExistingOptions, requested virtual.AttributesMask, a *virtual.Attributes) virtual.Status {
	if shareAccess&virtual.ShareMaskWrite != 0 {
		return virtual.StatusErrROFS
	}
	st, status := f.conn.getattr(f.path)
	if status != virtual.StatusOK {
		return status
	}
	cp := C.CString(f.path)
	defer C.free(unsafe.Pointer(cp))
	var fi C.struct_fuse_file_info
	f.conn.mu.Lock()
	r := C.call_open(f.conn.op, cp, &fi)
	f.conn.mu.Unlock()
	if r < 0 {
		return virtual.StatusErrIO
	}
	statToAttrs(&st, f.path, requested, a)
	return virtual.StatusOK
}

func (f *fuseFile) VirtualRead(buf []byte, offset uint64) (int, bool, virtual.Status) {
	if len(buf) == 0 {
		return 0, false, virtual.StatusOK
	}
	cp := C.CString(f.path)
	defer C.free(unsafe.Pointer(cp))
	var fi C.struct_fuse_file_info
	f.conn.mu.Lock()
	r := C.call_read(f.conn.op, cp, (*C.char)(unsafe.Pointer(&buf[0])), C.size_t(len(buf)), C.off_t(offset), &fi)
	f.conn.mu.Unlock()
	if r < 0 {
		return 0, false, virtual.StatusErrIO
	}
	n := int(r)
	// FUSE read returns the byte count; a short read means end of file.
	return n, n < len(buf), virtual.StatusOK
}

func (f *fuseFile) VirtualClose(shareAccess virtual.ShareMask) {}

func (f *fuseFile) VirtualSeek(offset uint64, regionType virtual.RegionType) (*uint64, virtual.Status) {
	st, status := f.conn.getattr(f.path)
	if status != virtual.StatusOK {
		return nil, virtual.StatusErrIO
	}
	size := uint64(st.st_size)
	if offset >= size {
		return nil, virtual.StatusErrNXIO
	}
	switch regionType {
	case virtual.Data:
		o := offset
		return &o, virtual.StatusOK
	case virtual.Hole:
		return &size, virtual.StatusOK
	default:
		return nil, virtual.StatusErrInval
	}
}

func (f *fuseFile) VirtualApply(data any) bool { return false }

func (f *fuseFile) VirtualSetAttributes(ctx context.Context, in *virtual.Attributes, requested virtual.AttributesMask, a *virtual.Attributes) virtual.Status {
	return virtual.StatusErrROFS
}
func (f *fuseFile) VirtualOpenNamedAttributes(ctx context.Context, createDirectory bool, requested virtual.AttributesMask, a *virtual.Attributes) (virtual.Directory, virtual.Status) {
	return nil, virtual.StatusErrNoEnt
}
func (f *fuseFile) VirtualAllocate(off, size uint64) virtual.Status { return virtual.StatusErrROFS }
func (f *fuseFile) VirtualWrite(buf []byte, offset uint64) (int, virtual.Status) {
	return 0, virtual.StatusErrROFS
}

func main() {}
