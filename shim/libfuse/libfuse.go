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
#include <time.h>
#include <sys/stat.h>

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

// --- write-path trampolines (Phase 2) ---
static int has_create(const struct fuse_operations *op) { return op->create != NULL; }
static int call_write(const struct fuse_operations *op, const char *path, const char *buf, size_t size, off_t off, struct fuse_file_info *fi) {
	if (!op->write) return -ENOSYS;
	return op->write(path, buf, size, off, fi);
}
static int call_create(const struct fuse_operations *op, const char *path, unsigned int mode, struct fuse_file_info *fi) {
	if (!op->create) return -ENOSYS;
	return op->create(path, (mode_t)mode, fi);
}
static int call_mknod(const struct fuse_operations *op, const char *path, unsigned int mode, unsigned long rdev) {
	if (!op->mknod) return -ENOSYS;
	return op->mknod(path, (mode_t)mode, (dev_t)rdev);
}
static int call_mkdir(const struct fuse_operations *op, const char *path, unsigned int mode) {
	if (!op->mkdir) return -ENOSYS;
	return op->mkdir(path, (mode_t)mode);
}
static int call_unlink(const struct fuse_operations *op, const char *path) {
	if (!op->unlink) return -ENOSYS;
	return op->unlink(path);
}
static int call_rmdir(const struct fuse_operations *op, const char *path) {
	if (!op->rmdir) return -ENOSYS;
	return op->rmdir(path);
}
static int call_rename(const struct fuse_operations *op, const char *from, const char *to) {
	if (!op->rename) return -ENOSYS;
	return op->rename(from, to);
}
static int call_truncate(const struct fuse_operations *op, const char *path, off_t size) {
	if (!op->truncate) return -ENOSYS;
	return op->truncate(path, size);
}
static int call_chmod(const struct fuse_operations *op, const char *path, unsigned int mode) {
	if (!op->chmod) return -ENOSYS;
	return op->chmod(path, (mode_t)mode);
}

// --- A1 structural ops: symlink / readlink / link ---
static int has_symlink(const struct fuse_operations *op) { return op->symlink != NULL; }
static int call_symlink(const struct fuse_operations *op, const char *target, const char *path) {
	if (!op->symlink) return -ENOSYS;
	return op->symlink(target, path);
}
// readlink: libfuse fills buf with a NUL-terminated target and returns 0 on
// success (NOT the length). buf must be at least `size` bytes; truncated if long.
static int call_readlink(const struct fuse_operations *op, const char *path, char *buf, size_t size) {
	if (!op->readlink) return -ENOSYS;
	return op->readlink(path, buf, size);
}
static int has_link(const struct fuse_operations *op) { return op->link != NULL; }
static int call_link(const struct fuse_operations *op, const char *from, const char *to) {
	if (!op->link) return -ENOSYS;
	return op->link(from, to);
}

// --- A1 time op: utimens. We only carry mtime (the FSAL has no atime), so
// atime is UTIME_OMIT — "leave unchanged" — and only mtime is set. ---
static int has_utimens(const struct fuse_operations *op) { return op->utimens != NULL; }
static int call_utimens(const struct fuse_operations *op, const char *path, long mt_sec, long mt_nsec) {
	if (!op->utimens) return -ENOSYS;
	struct timespec tv[2];
	tv[0].tv_sec = 0; tv[0].tv_nsec = UTIME_OMIT;       // atime: unchanged
	tv[1].tv_sec = mt_sec; tv[1].tv_nsec = mt_nsec;     // mtime: set
	return op->utimens(path, tv);
}

// --- lifecycle: init / destroy ---
// libfuse calls init() once before any operation; many filesystems (cgofuse's
// included) do their setup there, and init's return value becomes private_data.
static int has_init(const struct fuse_operations *op) { return op->init != NULL; }
static void *call_init(const struct fuse_operations *op) {
	struct fuse_conn_info conn;
	memset(&conn, 0, sizeof(conn));
	return op->init(&conn);
}
static void call_destroy(const struct fuse_operations *op, void *pd) {
	if (op->destroy) op->destroy(pd);
}

// galatea_set_user_data (defined in fuse_compat.c) updates what
// fuse_get_context()->private_data returns — used to install init()'s result.
void galatea_set_user_data(void *ud);

// readdir collection via a PURE C filler (no Go-export callback). The app's
// readdir calls c_filler, which appends NUL-terminated names into a caller-owned
// dirbuf; Go reads it after the call returns. This avoids handing the app a
// Go-exported function pointer — critical when the app is itself a Go program
// (e.g. cgofuse/rclone): a Go-export callback would re-enter *our* Go runtime
// from within the app's runtime, which Go cannot do (it faults; cgofuse reports
// it as EIO). A C filler keeps readdir to the same one-way shape getattr uses.
struct galatea_dirbuf { char names[65536]; size_t len; int count; int overflow; };
static int c_filler(void *buf, const char *name, const struct stat *stbuf, off_t off) {
	(void)stbuf; (void)off;
	struct galatea_dirbuf *db = (struct galatea_dirbuf *)buf;
	size_t n = strlen(name) + 1;
	if (db->len + n > sizeof(db->names)) { db->overflow = 1; return 1; } // buffer full
	memcpy(db->names + db->len, name, n);
	db->len += n;
	db->count++;
	return 0;
}
static int call_readdir(const struct fuse_operations *op, const char *path, struct galatea_dirbuf *db, struct fuse_file_info *fi) {
	if (!op->readdir) return -ENOSYS;
	return op->readdir(path, db, (fuse_fill_dir_t)c_filler, 0, fi);
}

// Handle-lifecycle trampolines. Stateful filesystems (cgofuse, real tools) hand
// out a file handle in opendir()/open() that readdir()/read()/write() then use,
// and free it in releasedir()/release(). Path-based filesystems (hello, the
// passthrough fixtures) ignore the handle. We bracket each data op with its
// open/release so a single fuse_file_info (carrying the handle) threads through —
// keeping the shim stateless while satisfying handle-based filesystems.
static int call_opendir(const struct fuse_operations *op, const char *path, struct fuse_file_info *fi) {
	if (!op->opendir) return 0;
	return op->opendir(path, fi);
}
static void call_releasedir(const struct fuse_operations *op, const char *path, struct fuse_file_info *fi) {
	if (op->releasedir) op->releasedir(path, fi);
}
static void call_release(const struct fuse_operations *op, const char *path, struct fuse_file_info *fi) {
	if (op->release) op->release(path, fi);
}
*/
import "C"

import (
	"bytes"
	"context"
	"fmt"
	"hash/fnv"
	"io"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	vpath "github.com/terraceonhigh/galatea/internal/bb/filesystem/path"
	nfssrv "github.com/terraceonhigh/galatea/internal/nfsv4"
	nfsproto "github.com/terraceonhigh/galatea/internal/xdr/pkg/protocols/nfsv4"
	"github.com/terraceonhigh/galatea/internal/xdr/pkg/rpcserver"
	"github.com/terraceonhigh/galatea/pkg/virtual"
)

// parserToString resolves a virtual.Parser (a path) back to its UNIX string
// form — the same Resolve+GetString round-trip the server uses to render
// READLINK responses (pathParserToLinktext4). Used to extract a symlink target
// from the parser the server hands VirtualSymlink.
func parserToString(p virtual.Parser) (string, bool) {
	builder, scopeWalker := vpath.EmptyBuilder.Join(vpath.VoidScopeWalker)
	if err := vpath.Resolve(p, scopeWalker); err != nil {
		return "", false
	}
	s, err := vpath.UNIXFormat.GetString(builder)
	if err != nil {
		return "", false
	}
	return s, true
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
	if os.Getenv("GALATEA_FUSE_TRACE") != "" {
		fmt.Fprintf(os.Stderr, "[shim] getattr(%q) -> r=%d mode=%#o size=%d\n", p, int(r), uint32(st.st_mode), int64(st.st_size))
	}
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
	cp := C.CString(p)
	defer C.free(unsafe.Pointer(cp))
	var db C.struct_galatea_dirbuf
	var fi C.struct_fuse_file_info
	c.mu.Lock()
	r := C.call_opendir(c.op, cp, &fi) // hands a handle to readdir; no-op for path-based fs's
	if r >= 0 {
		r = C.call_readdir(c.op, cp, &db, &fi)
		C.call_releasedir(c.op, cp, &fi)
	}
	c.mu.Unlock()
	if db.overflow != 0 {
		fmt.Fprintf(os.Stderr, "galatea-libfuse: readdir(%q): directory too large for buffer, entries truncated\n", p)
	}
	// db.names holds db.count NUL-terminated names back to back; "." and ".."
	// are dropped (the virtual layer doesn't expect them — see getattr("/.")).
	raw := C.GoBytes(unsafe.Pointer(&db.names[0]), C.int(db.len))
	var names []string
	for _, b := range bytes.Split(raw, []byte{0}) {
		n := string(b)
		if n == "" || n == "." || n == ".." {
			continue
		}
		names = append(names, n)
	}
	if os.Getenv("GALATEA_FUSE_TRACE") != "" {
		fmt.Fprintf(os.Stderr, "[shim] readdir(%q) -> r=%d names=%v\n", p, int(r), names)
	}
	if r != 0 {
		return nil, virtual.StatusErrIO
	}
	return names, virtual.StatusOK
}

// readlink calls the app's readlink under the lock and returns the target. The
// libfuse contract: the app NUL-terminates buf and returns 0 on success, so we
// read up to the first NUL (a fixed PATH_MAX-class buffer; longer targets are
// truncated, matching the kernel FUSE behaviour).
func (c *fuseConn) readlink(p string) (string, bool) {
	cp := C.CString(p)
	defer C.free(unsafe.Pointer(cp))
	buf := make([]byte, 4096)
	c.mu.Lock()
	r := C.call_readlink(c.op, cp, (*C.char)(unsafe.Pointer(&buf[0])), C.size_t(len(buf)))
	c.mu.Unlock()
	if r < 0 {
		return "", false
	}
	n := bytes.IndexByte(buf, 0)
	if n < 0 {
		n = len(buf)
	}
	return string(buf[:n]), true
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

// errStatus maps a FUSE op's return value (0 on success, -errno on failure) to a
// virtual.Status. A write/read returns a positive byte count, handled separately.
func errStatus(r C.int) virtual.Status {
	switch r {
	case 0:
		return virtual.StatusOK
	case -C.ENOENT:
		return virtual.StatusErrNoEnt
	case -C.EEXIST:
		return virtual.StatusErrExist
	case -C.ENOTEMPTY:
		return virtual.StatusErrNotEmpty
	case -C.EISDIR:
		return virtual.StatusErrIsDir
	case -C.ENOTDIR:
		return virtual.StatusErrNotDir
	case -C.EACCES:
		return virtual.StatusErrAccess
	case -C.EPERM:
		return virtual.StatusErrPerm
	case -C.EROFS:
		return virtual.StatusErrROFS
	default:
		return virtual.StatusErrIO
	}
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
	if requested&virtual.AttributesMaskLastDataModificationTime != 0 {
		// macOS struct stat carries the mtime as st_mtimespec (the POSIX
		// st_mtime is a macro over its tv_sec). Surfacing it lets a `touch`
		// (SETATTR→op->utimens) round-trip back through getattr to `ls -l`.
		mt := st.st_mtimespec
		a.SetLastDataModificationTime(time.Unix(int64(mt.tv_sec), int64(mt.tv_nsec)))
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
		if createAttributes == nil {
			return nil, 0, ci, virtual.StatusErrNoEnt
		}
		// Create a new regular file: prefer the app's create() (creates and
		// opens), else mknod() then open().
		mode := uint32(0o644)
		if perms, ok := createAttributes.GetPermissions(); ok {
			mode = perms.ToMode()
		}
		cp := C.CString(childPath)
		defer C.free(unsafe.Pointer(cp))
		var fi C.struct_fuse_file_info
		d.conn.mu.Lock()
		var r C.int
		if C.has_create(d.conn.op) != 0 {
			r = C.call_create(d.conn.op, cp, C.uint(mode), &fi)
		} else {
			r = C.call_mknod(d.conn.op, cp, C.uint(mode|uint32(C.S_IFREG)), 0)
			if r == 0 {
				r = C.call_open(d.conn.op, cp, &fi)
			}
		}
		d.conn.mu.Unlock()
		if r < 0 {
			return nil, 0, ci, errStatus(r)
		}
		// Neither create nor mknod returns a stat; getattr to fill the
		// attributes the client expects back from OPEN.
		st2, _ := d.conn.getattr(childPath)
		statToAttrs(&st2, childPath, requested, openedFileAttributes)
		return &fuseFile{conn: d.conn, path: childPath}, 0, ci, virtual.StatusOK
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
	// Existing file: open it (VirtualOpenSelf honors truncate), return attrs.
	leaf := &fuseFile{conn: d.conn, path: childPath}
	if s := leaf.VirtualOpenSelf(ctx, shareAccess, existingOptions, requested, openedFileAttributes); s != virtual.StatusOK {
		return nil, 0, ci, s
	}
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
	ci := virtual.ChangeInfo{}
	// The source must be a leaf from this same connection; a hard link across
	// filesystems is EXDEV, and a directory hard link is never allowed.
	src, ok := leaf.(*fuseFile)
	if !ok || src.conn != d.conn {
		return ci, virtual.StatusErrXDev
	}
	newPath := path.Join(d.path, name.String())
	cf := C.CString(src.path)
	defer C.free(unsafe.Pointer(cf))
	ct := C.CString(newPath)
	defer C.free(unsafe.Pointer(ct))
	d.conn.mu.Lock()
	r := C.call_link(d.conn.op, cf, ct)
	d.conn.mu.Unlock()
	if st := errStatus(r); st != virtual.StatusOK {
		return ci, st
	}
	if requested != 0 {
		st2, _ := d.conn.getattr(newPath)
		statToAttrs(&st2, newPath, requested, a)
	}
	return ci, virtual.StatusOK
}
func (d *fuseDir) VirtualMkdir(name virtual.Component, requested virtual.AttributesMask, a *virtual.Attributes) (virtual.Directory, virtual.ChangeInfo, virtual.Status) {
	ci := virtual.ChangeInfo{}
	childPath := path.Join(d.path, name.String())
	cp := C.CString(childPath)
	defer C.free(unsafe.Pointer(cp))
	d.conn.mu.Lock()
	r := C.call_mkdir(d.conn.op, cp, C.uint(0o755))
	d.conn.mu.Unlock()
	if st := errStatus(r); st != virtual.StatusOK {
		return nil, ci, st
	}
	st2, _ := d.conn.getattr(childPath)
	statToAttrs(&st2, childPath, requested, a)
	return &fuseDir{conn: d.conn, path: childPath}, ci, virtual.StatusOK
}
func (d *fuseDir) VirtualMknod(ctx context.Context, name virtual.Component, fileType virtual.FileType, requested virtual.AttributesMask, a *virtual.Attributes) (virtual.Leaf, virtual.ChangeInfo, virtual.Status) {
	return nil, virtual.ChangeInfo{}, virtual.StatusErrROFS
}
func (d *fuseDir) VirtualRename(oldName virtual.Component, newDirectory virtual.Directory, newName virtual.Component) (virtual.ChangeInfo, virtual.ChangeInfo, virtual.Status) {
	ci := virtual.ChangeInfo{}
	nd, ok := newDirectory.(*fuseDir)
	if !ok || nd.conn != d.conn {
		return ci, ci, virtual.StatusErrXDev
	}
	from := path.Join(d.path, oldName.String())
	to := path.Join(nd.path, newName.String())
	cf := C.CString(from)
	defer C.free(unsafe.Pointer(cf))
	ct := C.CString(to)
	defer C.free(unsafe.Pointer(ct))
	d.conn.mu.Lock()
	r := C.call_rename(d.conn.op, cf, ct)
	d.conn.mu.Unlock()
	return ci, ci, errStatus(r)
}
func (d *fuseDir) VirtualRemove(name virtual.Component, removeDirectory, removeLeaf bool) (virtual.ChangeInfo, virtual.Status) {
	ci := virtual.ChangeInfo{}
	childPath := path.Join(d.path, name.String())
	st, status := d.conn.getattr(childPath)
	if status != virtual.StatusOK {
		return ci, status
	}
	isDir := uint32(st.st_mode)&uint32(C.S_IFMT) == uint32(C.S_IFDIR)
	cp := C.CString(childPath)
	defer C.free(unsafe.Pointer(cp))
	d.conn.mu.Lock()
	defer d.conn.mu.Unlock()
	if isDir {
		if !removeDirectory {
			return ci, virtual.StatusErrIsDir
		}
		return ci, errStatus(C.call_rmdir(d.conn.op, cp))
	}
	if !removeLeaf {
		return ci, virtual.StatusErrNotDir
	}
	return ci, errStatus(C.call_unlink(d.conn.op, cp))
}
func (d *fuseDir) VirtualSymlink(ctx context.Context, pointedTo virtual.Parser, linkName virtual.Component, requested virtual.AttributesMask, a *virtual.Attributes) (virtual.Leaf, virtual.ChangeInfo, virtual.Status) {
	ci := virtual.ChangeInfo{}
	target, ok := parserToString(pointedTo)
	if !ok {
		return nil, ci, virtual.StatusErrInval
	}
	linkPath := path.Join(d.path, linkName.String())
	ct := C.CString(target)
	defer C.free(unsafe.Pointer(ct))
	cp := C.CString(linkPath)
	defer C.free(unsafe.Pointer(cp))
	d.conn.mu.Lock()
	r := C.call_symlink(d.conn.op, ct, cp) // libfuse symlink(target, linkpath)
	d.conn.mu.Unlock()
	if st := errStatus(r); st != virtual.StatusOK {
		return nil, ci, st
	}
	// opCreate(NF4LNK) reads the returned leaf's file handle back out of `a`
	// (it requested AttributesMaskFileHandle), so the attributes must be filled
	// — statToAttrs sets the handle from the path regardless of getattr success,
	// which matters for a dangling symlink whose target does not exist.
	leaf := &fuseFile{conn: d.conn, path: linkPath}
	st2, _ := d.conn.getattr(linkPath)
	statToAttrs(&st2, linkPath, requested, a)
	return leaf, ci, virtual.StatusOK
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
	// READLINK arrives as a GETATTR for the symlink-target attribute (the
	// server's opReadLink asks for AttributesMaskSymlinkTarget). Resolve it via
	// the app's readlink() only when the node is actually a symlink.
	if requested&virtual.AttributesMaskSymlinkTarget != 0 &&
		uint32(st.st_mode)&uint32(C.S_IFMT) == uint32(C.S_IFLNK) {
		if target, ok := f.conn.readlink(f.path); ok {
			a.SetSymlinkTarget(vpath.UNIXFormat.NewParser(target))
		}
	}
}

func (f *fuseFile) VirtualOpenSelf(ctx context.Context, shareAccess virtual.ShareMask, options *virtual.OpenExistingOptions, requested virtual.AttributesMask, a *virtual.Attributes) virtual.Status {
	if options != nil && options.Truncate { // open-with-truncate, e.g. shell `>`
		cp := C.CString(f.path)
		f.conn.mu.Lock()
		r := C.call_truncate(f.conn.op, cp, 0)
		f.conn.mu.Unlock()
		C.free(unsafe.Pointer(cp))
		if r < 0 {
			return errStatus(r)
		}
	}
	// No op->open here: VirtualRead/VirtualWrite bracket each data op with their
	// own open/release, so a handle-based fs gets a valid fh per op and we don't
	// leak its open handle across the NFS open lifecycle.
	st, status := f.conn.getattr(f.path)
	if status != virtual.StatusOK {
		return status
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
	r := C.call_open(f.conn.op, cp, &fi) // open → read(fh) → release, one fi
	if r >= 0 {
		r = C.call_read(f.conn.op, cp, (*C.char)(unsafe.Pointer(&buf[0])), C.size_t(len(buf)), C.off_t(offset), &fi)
		C.call_release(f.conn.op, cp, &fi)
	}
	f.conn.mu.Unlock()
	if r < 0 {
		return 0, false, errStatus(r)
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
	cp := C.CString(f.path)
	defer C.free(unsafe.Pointer(cp))
	// Apply what `in` carries (its fieldsPresent), NOT what `requested` asks to
	// read back — keying off the return mask was the truncate bug (memory.go).
	if size, ok := in.GetSizeBytes(); ok {
		f.conn.mu.Lock()
		r := C.call_truncate(f.conn.op, cp, C.off_t(size))
		f.conn.mu.Unlock()
		if r < 0 {
			return errStatus(r)
		}
	}
	if perms, ok := in.GetPermissions(); ok {
		f.conn.mu.Lock()
		r := C.call_chmod(f.conn.op, cp, C.uint(perms.ToMode()))
		f.conn.mu.Unlock()
		if r < 0 {
			return errStatus(r)
		}
	}
	if mtime, ok := in.GetLastDataModificationTime(); ok {
		// SETATTR time_modify_set → op->utimens (mtime only; atime is UTIME_OMIT
		// in the trampoline, since the FSAL carries no atime).
		f.conn.mu.Lock()
		r := C.call_utimens(f.conn.op, cp, C.long(mtime.Unix()), C.long(mtime.Nanosecond()))
		f.conn.mu.Unlock()
		if r < 0 {
			return errStatus(r)
		}
	}
	st, status := f.conn.getattr(f.path)
	if status != virtual.StatusOK {
		return status
	}
	statToAttrs(&st, f.path, requested, a)
	return virtual.StatusOK
}
func (f *fuseFile) VirtualOpenNamedAttributes(ctx context.Context, createDirectory bool, requested virtual.AttributesMask, a *virtual.Attributes) (virtual.Directory, virtual.Status) {
	return nil, virtual.StatusErrNoEnt
}
func (f *fuseFile) VirtualAllocate(off, size uint64) virtual.Status { return virtual.StatusErrROFS }
func (f *fuseFile) VirtualWrite(buf []byte, offset uint64) (int, virtual.Status) {
	if len(buf) == 0 {
		return 0, virtual.StatusOK
	}
	cp := C.CString(f.path)
	defer C.free(unsafe.Pointer(cp))
	var fi C.struct_fuse_file_info
	f.conn.mu.Lock()
	r := C.call_open(f.conn.op, cp, &fi) // open → write(fh) → release, one fi
	if r >= 0 {
		r = C.call_write(f.conn.op, cp, (*C.char)(unsafe.Pointer(&buf[0])), C.size_t(len(buf)), C.off_t(offset), &fi)
		C.call_release(f.conn.op, cp, &fi)
	}
	f.conn.mu.Unlock()
	if r < 0 {
		return 0, errStatus(r)
	}
	return int(r), virtual.StatusOK
}

// goArgs converts a C argv (argc, char**) into a Go []string.
func goArgs(argc C.int, argv **C.char) []string {
	n := int(argc)
	if n <= 0 {
		return nil
	}
	p := unsafe.Slice(argv, n)
	out := make([]string, n)
	for i := 0; i < n; i++ {
		out[i] = C.GoString(p[i])
	}
	return out
}

// galateaFuseMain is the Go body behind the fuse_main(...) macro every libfuse
// app calls. The C-ABI symbol fuse_main_real itself is defined in bridge.c
// (matching fuse.h's exact signature) and forwards here — exporting a Go
// function *named* fuse_main_real would clash with fuse.h's own declaration.
// Instead of a kernel FUSE driver, this stands Galatea's NFSv4 server up over
// the app's operations and mounts it with the stock macOS NFS client. Blocks
// until SIGINT/SIGTERM, then unmounts — for the spike we don't implement
// unmount-detection (fuse_main's "return on unmount" contract); a signal is the
// clean teardown. op is the app's struct fuse_operations*, taken as
// unsafe.Pointer (a typed C pointer isn't assignable across the bridge.c/Go cgo
// boundary).
//
//export galateaFuseMain
func galateaFuseMain(argc C.int, argv **C.char, op unsafe.Pointer) C.int {
	args := goArgs(argc, argv)
	// The mountpoint is the last non-option argument (a simplification that
	// holds for hello.c and most simple invocations; full fuse_opt parsing is
	// later work).
	mountpoint := ""
	for _, a := range args[1:] {
		if !strings.HasPrefix(a, "-") {
			mountpoint = a
		}
	}
	if mountpoint == "" {
		fmt.Fprintln(os.Stderr, "galatea-libfuse: no mountpoint found in argv")
		return 1
	}

	cop := (*C.struct_fuse_operations)(op)
	// libfuse calls init() before any operation; cgofuse (and many filesystems)
	// build their state there, and init's return value replaces private_data.
	// Run it before the mount, so the first client op sees an initialised fs.
	var privData unsafe.Pointer
	if C.has_init(cop) != 0 {
		privData = C.call_init(cop)
		if privData != nil {
			C.galatea_set_user_data(privData)
		}
	}

	root, resolver := NewFuseRoot(op)
	var program nfsproto.Nfs4Program = nfssrv.NewReadOnlyProgram(root, resolver)
	server := rpcserver.NewServer(
		map[uint32]rpcserver.Service{
			nfsproto.NFS4_PROGRAM_PROGRAM_NUMBER: nfsproto.NewNfs4ProgramService(program),
		},
		nfssrv.NewSystemAuthenticator(),
	)

	// Bind + listen synchronously, and start accepting, before mount_nfs — the
	// client must find a listening, accepting server when it connects.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		fmt.Fprintf(os.Stderr, "galatea-libfuse: listen: %v\n", err)
		return 1
	}
	port := ln.Addr().(*net.TCPAddr).Port
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				_ = server.HandleConnection(conn, conn)
				_ = conn.Close()
			}()
		}
	}()

	fmt.Printf("galatea-libfuse: serving NFSv4 on 127.0.0.1:%d → mounting at %s\n", port, mountpoint)
	mnt := exec.Command("mount_nfs", "-o",
		fmt.Sprintf("vers=4.0,port=%d,mountport=%d,tcp", port, port),
		"localhost:/", mountpoint)
	mnt.Stdout, mnt.Stderr = os.Stdout, os.Stderr
	if err := mnt.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "galatea-libfuse: mount_nfs: %v\n", err)
		_ = ln.Close()
		return 1
	}
	fmt.Printf("galatea-libfuse: mounted at %s (Ctrl-C to unmount)\n", mountpoint)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	<-sig
	fmt.Println("\ngalatea-libfuse: signal received, unmounting")
	_ = exec.Command("umount", mountpoint).Run()
	C.call_destroy(cop, privData) // libfuse calls destroy() with private_data at teardown
	_ = ln.Close()
	return 0
}

func main() {}
