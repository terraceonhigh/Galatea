// Package osfs is a read-only Galatea FSAL backed by the local operating
// system filesystem. It maps os.ReadDir / os.Open / os.Stat onto the
// virtual.Directory / virtual.Leaf interface, so a real directory tree
// can be navigated through Galatea's filesystem abstraction layer.
//
// It is the first Galatea backend over an externally-defined data source
// (the in-memory FSAL is a test fixture); see docs/DECISIONS.md DEC-006.
// Read-only by design: every mutating operation returns StatusErrROFS.
package osfs

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"sort"
	"syscall"

	"github.com/terraceonhigh/galatea/pkg/virtual"
)

// Root returns a virtual.Directory rooted at the given host path. The
// path must name an existing directory.
func Root(hostPath string) (virtual.Directory, error) {
	abs, err := filepath.Abs(hostPath)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, &os.PathError{Op: "root", Path: abs, Err: syscall.ENOTDIR}
	}
	return &directory{path: abs}, nil
}

// fileTypeOf maps an os.FileMode to a virtual.FileType.
func fileTypeOf(mode os.FileMode) virtual.FileType {
	switch {
	case mode&os.ModeDir != 0:
		return virtual.FileTypeDirectory
	case mode&os.ModeSymlink != 0:
		return virtual.FileTypeSymlink
	case mode&os.ModeNamedPipe != 0:
		return virtual.FileTypeFIFO
	case mode&os.ModeSocket != 0:
		return virtual.FileTypeSocket
	case mode&os.ModeCharDevice != 0:
		return virtual.FileTypeCharacterDevice
	case mode&os.ModeDevice != 0:
		return virtual.FileTypeBlockDevice
	case mode.IsRegular():
		return virtual.FileTypeRegularFile
	default:
		return virtual.FileTypeOther
	}
}

// fillAttributes populates the requested attributes from an os.FileInfo.
func fillAttributes(fi os.FileInfo, requested virtual.AttributesMask, a *virtual.Attributes) {
	if requested&virtual.AttributesMaskFileType != 0 {
		a.SetFileType(fileTypeOf(fi.Mode()))
	}
	if requested&virtual.AttributesMaskPermissions != 0 {
		a.SetPermissions(virtual.NewPermissionsFromMode(uint32(fi.Mode().Perm())))
	}
	if requested&virtual.AttributesMaskSizeBytes != 0 {
		size := fi.Size()
		if size < 0 {
			size = 0
		}
		a.SetSizeBytes(uint64(size))
	}
	if requested&virtual.AttributesMaskLastDataModificationTime != 0 {
		a.SetLastDataModificationTime(fi.ModTime())
	}
	var ino uint64
	var nlink uint32 = 1
	if st, ok := fi.Sys().(*syscall.Stat_t); ok {
		ino = st.Ino
		nlink = uint32(st.Nlink)
	}
	if requested&virtual.AttributesMaskInodeNumber != 0 {
		a.SetInodeNumber(ino)
	}
	if requested&virtual.AttributesMaskLinkCount != 0 {
		a.SetLinkCount(nlink)
	}
	if requested&virtual.AttributesMaskChangeID != 0 {
		// Use the modification time (nanoseconds) as a change ID: it
		// advances whenever the file's data changes.
		a.SetChangeID(uint64(fi.ModTime().UnixNano()))
	}
}

// nodeFor builds the virtual node (directory or leaf) for a host path,
// given its already-resolved FileInfo.
func nodeFor(path string, fi os.FileInfo) virtual.Node {
	if fi.IsDir() {
		return &directory{path: path}
	}
	return &file{path: path}
}

// --- directory ------------------------------------------------------------

type directory struct {
	path string
}

func (d *directory) VirtualGetAttributes(ctx context.Context, requested virtual.AttributesMask, attributes *virtual.Attributes) {
	fi, err := os.Stat(d.path)
	if err != nil {
		// A directory that vanished mid-traversal: report an empty,
		// type-only best effort.
		if requested&virtual.AttributesMaskFileType != 0 {
			attributes.SetFileType(virtual.FileTypeDirectory)
		}
		return
	}
	fillAttributes(fi, requested, attributes)
}

func (d *directory) VirtualSetAttributes(ctx context.Context, in *virtual.Attributes, requested virtual.AttributesMask, attributes *virtual.Attributes) virtual.Status {
	return virtual.StatusErrROFS
}

func (d *directory) VirtualApply(data any) bool { return false }

func (d *directory) VirtualOpenNamedAttributes(ctx context.Context, createDirectory bool, requested virtual.AttributesMask, attributes *virtual.Attributes) (virtual.Directory, virtual.Status) {
	return nil, virtual.StatusErrNoEnt
}

func (d *directory) VirtualLookup(ctx context.Context, name virtual.Component, requested virtual.AttributesMask, out *virtual.Attributes) (virtual.DirectoryChild, virtual.Status) {
	childPath := filepath.Join(d.path, name.String())
	fi, err := os.Lstat(childPath)
	if err != nil {
		if os.IsNotExist(err) {
			return virtual.DirectoryChild{}, virtual.StatusErrNoEnt
		}
		return virtual.DirectoryChild{}, virtual.StatusErrIO
	}
	node := nodeFor(childPath, fi)
	fillAttributes(fi, requested, out)
	var c virtual.DirectoryChild
	if dir, ok := node.(virtual.Directory); ok {
		return c.FromDirectory(dir), virtual.StatusOK
	}
	return c.FromLeaf(node.(virtual.Leaf)), virtual.StatusOK
}

func (d *directory) VirtualReadDir(ctx context.Context, firstCookie uint64, requested virtual.AttributesMask, reporter virtual.DirectoryEntryReporter) virtual.Status {
	entries, err := os.ReadDir(d.path)
	if err != nil {
		return virtual.StatusErrIO
	}
	// os.ReadDir already returns entries sorted by name; cookies are
	// 1-based indices into that stable order.
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	for i, entry := range entries {
		cookie := uint64(i + 1)
		if cookie <= firstCookie {
			continue
		}
		name, ok := virtual.NewComponent(entry.Name())
		if !ok {
			// Skip names that are not valid components (should not
			// happen for real on-disk names, but stay defensive).
			continue
		}
		fi, err := entry.Info()
		if err != nil {
			continue
		}
		childPath := filepath.Join(d.path, entry.Name())
		node := nodeFor(childPath, fi)
		var attributes virtual.Attributes
		fillAttributes(fi, requested, &attributes)
		var c virtual.DirectoryChild
		if dir, ok := node.(virtual.Directory); ok {
			c = c.FromDirectory(dir)
		} else {
			c = c.FromLeaf(node.(virtual.Leaf))
		}
		if !reporter.ReportEntry(cookie, name, c, &attributes) {
			break
		}
	}
	return virtual.StatusOK
}

func (d *directory) VirtualOpenChild(ctx context.Context, name virtual.Component, shareAccess virtual.ShareMask, createAttributes *virtual.Attributes, existingOptions *virtual.OpenExistingOptions, requested virtual.AttributesMask, openedFileAttributes *virtual.Attributes) (virtual.Leaf, virtual.AttributesMask, virtual.ChangeInfo, virtual.Status) {
	ci := virtual.ChangeInfo{}
	childPath := filepath.Join(d.path, name.String())
	fi, err := os.Stat(childPath)
	if err != nil {
		if os.IsNotExist(err) {
			if createAttributes != nil {
				return nil, 0, ci, virtual.StatusErrROFS
			}
			return nil, 0, ci, virtual.StatusErrNoEnt
		}
		return nil, 0, ci, virtual.StatusErrIO
	}
	if existingOptions == nil {
		return nil, 0, ci, virtual.StatusErrExist
	}
	if fi.IsDir() {
		return nil, 0, ci, virtual.StatusErrIsDir
	}
	if shareAccess&virtual.ShareMaskWrite != 0 {
		return nil, 0, ci, virtual.StatusErrROFS
	}
	leaf := &file{path: childPath}
	fillAttributes(fi, requested, openedFileAttributes)
	return leaf, 0, ci, virtual.StatusOK
}

func (d *directory) VirtualLink(ctx context.Context, name virtual.Component, leaf virtual.Leaf, requested virtual.AttributesMask, attributes *virtual.Attributes) (virtual.ChangeInfo, virtual.Status) {
	return virtual.ChangeInfo{}, virtual.StatusErrROFS
}

func (d *directory) VirtualMkdir(name virtual.Component, requested virtual.AttributesMask, attributes *virtual.Attributes) (virtual.Directory, virtual.ChangeInfo, virtual.Status) {
	return nil, virtual.ChangeInfo{}, virtual.StatusErrROFS
}

func (d *directory) VirtualMknod(ctx context.Context, name virtual.Component, fileType virtual.FileType, requested virtual.AttributesMask, attributes *virtual.Attributes) (virtual.Leaf, virtual.ChangeInfo, virtual.Status) {
	return nil, virtual.ChangeInfo{}, virtual.StatusErrROFS
}

func (d *directory) VirtualRename(oldName virtual.Component, newDirectory virtual.Directory, newName virtual.Component) (virtual.ChangeInfo, virtual.ChangeInfo, virtual.Status) {
	return virtual.ChangeInfo{}, virtual.ChangeInfo{}, virtual.StatusErrROFS
}

func (d *directory) VirtualRemove(name virtual.Component, removeDirectory, removeLeaf bool) (virtual.ChangeInfo, virtual.Status) {
	return virtual.ChangeInfo{}, virtual.StatusErrROFS
}

func (d *directory) VirtualSymlink(ctx context.Context, pointedTo virtual.Parser, linkName virtual.Component, requested virtual.AttributesMask, attributes *virtual.Attributes) (virtual.Leaf, virtual.ChangeInfo, virtual.Status) {
	return nil, virtual.ChangeInfo{}, virtual.StatusErrROFS
}

// --- file -----------------------------------------------------------------

type file struct {
	path string
}

func (f *file) VirtualGetAttributes(ctx context.Context, requested virtual.AttributesMask, attributes *virtual.Attributes) {
	fi, err := os.Lstat(f.path)
	if err != nil {
		if requested&virtual.AttributesMaskFileType != 0 {
			attributes.SetFileType(virtual.FileTypeRegularFile)
		}
		return
	}
	fillAttributes(fi, requested, attributes)
}

func (f *file) VirtualSetAttributes(ctx context.Context, in *virtual.Attributes, requested virtual.AttributesMask, attributes *virtual.Attributes) virtual.Status {
	return virtual.StatusErrROFS
}

func (f *file) VirtualApply(data any) bool { return false }

func (f *file) VirtualOpenNamedAttributes(ctx context.Context, createDirectory bool, requested virtual.AttributesMask, attributes *virtual.Attributes) (virtual.Directory, virtual.Status) {
	return nil, virtual.StatusErrNoEnt
}

func (f *file) VirtualAllocate(off, size uint64) virtual.Status { return virtual.StatusErrROFS }

func (f *file) VirtualSeek(offset uint64, regionType virtual.RegionType) (*uint64, virtual.Status) {
	fi, err := os.Stat(f.path)
	if err != nil {
		return nil, virtual.StatusErrIO
	}
	size := uint64(fi.Size())
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

func (f *file) VirtualOpenSelf(ctx context.Context, shareAccess virtual.ShareMask, options *virtual.OpenExistingOptions, requested virtual.AttributesMask, attributes *virtual.Attributes) virtual.Status {
	if shareAccess&virtual.ShareMaskWrite != 0 {
		return virtual.StatusErrROFS
	}
	fi, err := os.Stat(f.path)
	if err != nil {
		return virtual.StatusErrIO
	}
	fillAttributes(fi, requested, attributes)
	return virtual.StatusOK
}

func (f *file) VirtualRead(buf []byte, offset uint64) (int, bool, virtual.Status) {
	fh, err := os.Open(f.path)
	if err != nil {
		return 0, false, virtual.StatusErrIO
	}
	defer fh.Close()
	n, err := fh.ReadAt(buf, int64(offset))
	if err != nil && err != io.EOF {
		return 0, false, virtual.StatusErrIO
	}
	return n, err == io.EOF, virtual.StatusOK
}

func (f *file) VirtualClose(shareAccess virtual.ShareMask) {}

func (f *file) VirtualWrite(buf []byte, offset uint64) (int, virtual.Status) {
	return 0, virtual.StatusErrROFS
}
