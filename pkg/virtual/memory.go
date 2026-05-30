package virtual

import (
	"context"
	"encoding/binary"
	"io"
	"sort"
	"sync"
)

// NewMemoryHandleResolver builds a HandleResolver (DEC-017 Option B) over the
// in-memory tree rooted at root, which must be an in-memory directory. It walks
// the tree once, indexing every node by its inode, and returns a resolver that
// decodes the 8-byte inode handle (see memoryFileHandle) back to its node. The
// NFSv4 server needs this so a client's PUTFH — replaying a handle it obtained
// from an earlier GETFH — resolves to the right node.
//
// The index is a snapshot taken at construction. That is correct for this
// read-only FSAL (the tree is immutable); a mutable backend would maintain the
// index as nodes come and go.
func NewMemoryHandleResolver(root Directory) HandleResolver {
	index := map[uint64]Node{}
	var walk func(n Node)
	walk = func(n Node) {
		switch t := n.(type) {
		case *memoryDirectory:
			index[t.inode] = t
			for _, c := range t.children {
				walk(c)
			}
		case *memoryFile:
			index[t.inode] = t
		}
	}
	walk(root)

	return func(r io.ByteReader) (DirectoryChild, Status) {
		var h [8]byte
		for i := range h {
			b, err := r.ReadByte()
			if err != nil {
				return DirectoryChild{}, StatusErrBadHandle
			}
			h[i] = b
		}
		n, ok := index[binary.BigEndian.Uint64(h[:])]
		if !ok {
			return DirectoryChild{}, StatusErrStale
		}
		return childOf(n), StatusOK
	}
}

// memoryFileHandle encodes a node's stable inode number as an 8-byte NFS file
// handle. This is the in-memory FSAL's half of DEC-017's Option B (backends
// self-assign handles): the server reads it via AttributesMaskFileHandle, and a
// resolver (handle → node) decodes the inode back. Big-endian so handles sort
// the way inodes do.
func memoryFileHandle(inode uint64) []byte {
	h := make([]byte, 8)
	binary.BigEndian.PutUint64(h, inode)
	return h
}

// This file is a minimal, read-only in-memory FSAL. It exists to prove
// the interface in pkg/virtual is implementable and exercisable without
// any backend — the trivial test FSAL DEC-005 and the charge letter
// (Phase 1) call for before a real backend or the lifted NFSv4 server
// exist. Mutating operations return StatusErrROFS; reads work.

// childOf builds a DirectoryChild from a Node, dispatching on whether it
// is a Directory or a Leaf.
func childOf(n Node) DirectoryChild {
	var c DirectoryChild
	if d, ok := n.(Directory); ok {
		return c.FromDirectory(d)
	}
	if l, ok := n.(Leaf); ok {
		return c.FromLeaf(l)
	}
	panic("node is neither a Directory nor a Leaf")
}

// --- memoryFile: a writable regular file ----------------------------------

// memoryFile is an in-memory regular file. Its contents and size are mutable
// (R6 write path); a per-file mutex serializes reads, writes, truncations, and
// attribute reads of this file. Directory structure is a separate concern (the
// directory's own concurrency); a memoryFile only guards its own bytes.
type memoryFile struct {
	mu       sync.Mutex
	inode    uint64
	perms    Permissions
	contents []byte
}

// NewMemoryFile returns an in-memory regular file with the given initial
// contents. The file is writable (see VirtualWrite / VirtualSetAttributes).
func NewMemoryFile(inode uint64, perms Permissions, contents []byte) Leaf {
	return &memoryFile{inode: inode, perms: perms, contents: contents}
}

// fillAttributes writes the requested attributes. The caller must hold f.mu
// (it reads mutable size/perms).
func (f *memoryFile) fillAttributes(requested AttributesMask, a *Attributes) {
	if requested&AttributesMaskFileType != 0 {
		a.SetFileType(FileTypeRegularFile)
	}
	if requested&AttributesMaskPermissions != 0 {
		a.SetPermissions(f.perms)
	}
	if requested&AttributesMaskSizeBytes != 0 {
		a.SetSizeBytes(uint64(len(f.contents)))
	}
	if requested&AttributesMaskInodeNumber != 0 {
		a.SetInodeNumber(f.inode)
	}
	if requested&AttributesMaskFileHandle != 0 {
		a.SetFileHandle(memoryFileHandle(f.inode))
	}
	if requested&AttributesMaskHasNamedAttributes != 0 {
		a.SetHasNamedAttributes(false)
	}
	if requested&AttributesMaskIsInNamedAttributeDirectory != 0 {
		// This FSAL has no named-attribute directories.
		a.SetIsInNamedAttributeDirectory(false)
	}
	if requested&AttributesMaskLinkCount != 0 {
		a.SetLinkCount(1)
	}
	if requested&AttributesMaskChangeID != 0 {
		a.SetChangeID(0)
	}
}

func (f *memoryFile) VirtualGetAttributes(ctx context.Context, requested AttributesMask, attributes *Attributes) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.fillAttributes(requested, attributes)
}

// resize truncates or zero-extends the contents to size. Caller holds f.mu.
func (f *memoryFile) resize(size uint64) {
	switch n := uint64(len(f.contents)); {
	case size < n:
		f.contents = f.contents[:size]
	case size > n:
		f.contents = append(f.contents, make([]byte, size-n)...)
	}
}

func (f *memoryFile) VirtualSetAttributes(ctx context.Context, in *Attributes, requested AttributesMask, attributes *Attributes) Status {
	f.mu.Lock()
	defer f.mu.Unlock()
	if requested&AttributesMaskSizeBytes != 0 {
		if size, ok := in.GetSizeBytes(); ok {
			f.resize(size)
		}
	}
	if requested&AttributesMaskPermissions != 0 {
		if perms, ok := in.GetPermissions(); ok {
			f.perms = perms
		}
	}
	f.fillAttributes(requested, attributes)
	return StatusOK
}

func (f *memoryFile) VirtualApply(data any) bool { return false }

func (f *memoryFile) VirtualOpenNamedAttributes(ctx context.Context, createDirectory bool, requested AttributesMask, attributes *Attributes) (Directory, Status) {
	return nil, StatusErrNoEnt
}

func (f *memoryFile) VirtualAllocate(off, size uint64) Status {
	f.mu.Lock()
	defer f.mu.Unlock()
	if end := off + size; end > uint64(len(f.contents)) {
		f.resize(end)
	}
	return StatusOK
}

func (f *memoryFile) VirtualSeek(offset uint64, regionType RegionType) (*uint64, Status) {
	f.mu.Lock()
	defer f.mu.Unlock()
	size := uint64(len(f.contents))
	if offset >= size {
		return nil, StatusErrNXIO
	}
	switch regionType {
	case Data:
		// The whole file is data; the next data region starts here.
		o := offset
		return &o, StatusOK
	case Hole:
		// No holes; the only "hole" is the implicit one at EOF.
		return &size, StatusOK
	default:
		return nil, StatusErrInval
	}
}

func (f *memoryFile) VirtualOpenSelf(ctx context.Context, shareAccess ShareMask, options *OpenExistingOptions, requested AttributesMask, attributes *Attributes) Status {
	f.mu.Lock()
	defer f.mu.Unlock()
	if options != nil && options.Truncate {
		// Open-with-truncate (e.g. shell `>`): empty the file first.
		f.resize(0)
	}
	f.fillAttributes(requested, attributes)
	return StatusOK
}

func (f *memoryFile) VirtualRead(buf []byte, offset uint64) (int, bool, Status) {
	f.mu.Lock()
	defer f.mu.Unlock()
	size := uint64(len(f.contents))
	if offset >= size {
		// At or past end-of-file: nothing to read. An NFS client (and
		// pjdfstest) will issue such reads; they must report EOF, not
		// slice out of range.
		return 0, true, StatusOK
	}
	data, eof := BoundReadToFileSize(buf, offset, size)
	n := copy(data, f.contents[offset:])
	return n, eof, StatusOK
}

func (f *memoryFile) VirtualClose(shareAccess ShareMask) {}

// VirtualWrite writes buf at offset, zero-extending the file first if the write
// starts past the current end.
func (f *memoryFile) VirtualWrite(buf []byte, offset uint64) (int, Status) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if end := offset + uint64(len(buf)); end > uint64(len(f.contents)) {
		f.resize(end)
	}
	copy(f.contents[offset:], buf)
	return len(buf), StatusOK
}

// --- memoryDirectory: a read-only directory -------------------------------

type memoryDirectory struct {
	inode    uint64
	perms    Permissions
	children map[string]Node
}

// NewMemoryDirectory returns a read-only in-memory directory. Each value
// in children must itself be a *memoryFile or *memoryDirectory (i.e. a
// Leaf or a Directory).
func NewMemoryDirectory(inode uint64, perms Permissions, children map[string]Node) Directory {
	if children == nil {
		children = map[string]Node{}
	}
	return &memoryDirectory{inode: inode, perms: perms, children: children}
}

func (d *memoryDirectory) fillAttributes(requested AttributesMask, a *Attributes) {
	if requested&AttributesMaskFileType != 0 {
		a.SetFileType(FileTypeDirectory)
	}
	if requested&AttributesMaskPermissions != 0 {
		a.SetPermissions(d.perms)
	}
	if requested&AttributesMaskSizeBytes != 0 {
		a.SetSizeBytes(0)
	}
	if requested&AttributesMaskInodeNumber != 0 {
		a.SetInodeNumber(d.inode)
	}
	if requested&AttributesMaskFileHandle != 0 {
		a.SetFileHandle(memoryFileHandle(d.inode))
	}
	if requested&AttributesMaskHasNamedAttributes != 0 {
		a.SetHasNamedAttributes(false)
	}
	if requested&AttributesMaskIsInNamedAttributeDirectory != 0 {
		// This FSAL has no named-attribute directories.
		a.SetIsInNamedAttributeDirectory(false)
	}
	if requested&AttributesMaskLinkCount != 0 {
		a.SetLinkCount(EmptyDirectoryLinkCount)
	}
	if requested&AttributesMaskChangeID != 0 {
		a.SetChangeID(0)
	}
}

func (d *memoryDirectory) VirtualGetAttributes(ctx context.Context, requested AttributesMask, attributes *Attributes) {
	d.fillAttributes(requested, attributes)
}

func (d *memoryDirectory) VirtualSetAttributes(ctx context.Context, in *Attributes, requested AttributesMask, attributes *Attributes) Status {
	return StatusErrROFS
}

func (d *memoryDirectory) VirtualApply(data any) bool { return false }

func (d *memoryDirectory) VirtualOpenNamedAttributes(ctx context.Context, createDirectory bool, requested AttributesMask, attributes *Attributes) (Directory, Status) {
	return nil, StatusErrNoEnt
}

func (d *memoryDirectory) VirtualOpenChild(ctx context.Context, name Component, shareAccess ShareMask, createAttributes *Attributes, existingOptions *OpenExistingOptions, requested AttributesMask, openedFileAttributes *Attributes) (Leaf, AttributesMask, ChangeInfo, Status) {
	ci := ChangeInfo{}
	child, ok := d.children[name.String()]
	if !ok {
		if createAttributes != nil {
			// Would create — but this FSAL is read-only.
			return nil, 0, ci, StatusErrROFS
		}
		return nil, 0, ci, StatusErrNoEnt
	}
	if existingOptions == nil {
		// Caller asked to create-only, but the file exists.
		return nil, 0, ci, StatusErrExist
	}
	leaf, ok := child.(Leaf)
	if !ok {
		return nil, 0, ci, StatusErrIsDir
	}
	if s := leaf.VirtualOpenSelf(ctx, shareAccess, existingOptions, requested, openedFileAttributes); s != StatusOK {
		return nil, 0, ci, s
	}
	return leaf, 0, ci, StatusOK
}

func (d *memoryDirectory) VirtualLink(ctx context.Context, name Component, leaf Leaf, requested AttributesMask, attributes *Attributes) (ChangeInfo, Status) {
	return ChangeInfo{}, StatusErrROFS
}

func (d *memoryDirectory) VirtualLookup(ctx context.Context, name Component, requested AttributesMask, out *Attributes) (DirectoryChild, Status) {
	child, ok := d.children[name.String()]
	if !ok {
		return DirectoryChild{}, StatusErrNoEnt
	}
	child.VirtualGetAttributes(ctx, requested, out)
	return childOf(child), StatusOK
}

func (d *memoryDirectory) VirtualMkdir(name Component, requested AttributesMask, attributes *Attributes) (Directory, ChangeInfo, Status) {
	return nil, ChangeInfo{}, StatusErrROFS
}

func (d *memoryDirectory) VirtualMknod(ctx context.Context, name Component, fileType FileType, requested AttributesMask, attributes *Attributes) (Leaf, ChangeInfo, Status) {
	return nil, ChangeInfo{}, StatusErrROFS
}

func (d *memoryDirectory) VirtualReadDir(ctx context.Context, firstCookie uint64, requested AttributesMask, reporter DirectoryEntryReporter) Status {
	names := make([]string, 0, len(d.children))
	for name := range d.children {
		names = append(names, name)
	}
	sort.Strings(names)
	for i, name := range names {
		cookie := uint64(i + 1)
		if cookie <= firstCookie {
			continue
		}
		child := d.children[name]
		var attributes Attributes
		child.VirtualGetAttributes(ctx, requested, &attributes)
		component, ok := NewComponent(name)
		if !ok {
			return StatusErrIO
		}
		if !reporter.ReportEntry(cookie, component, childOf(child), &attributes) {
			break
		}
	}
	return StatusOK
}

func (d *memoryDirectory) VirtualRename(oldName Component, newDirectory Directory, newName Component) (ChangeInfo, ChangeInfo, Status) {
	return ChangeInfo{}, ChangeInfo{}, StatusErrROFS
}

func (d *memoryDirectory) VirtualRemove(name Component, removeDirectory, removeLeaf bool) (ChangeInfo, Status) {
	return ChangeInfo{}, StatusErrROFS
}

func (d *memoryDirectory) VirtualSymlink(ctx context.Context, pointedTo Parser, linkName Component, requested AttributesMask, attributes *Attributes) (Leaf, ChangeInfo, Status) {
	return nil, ChangeInfo{}, StatusErrROFS
}
