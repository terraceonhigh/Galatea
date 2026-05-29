package virtual

import "strings"

// This file holds the leaf types Galatea provides in place of the
// bb-storage types bb-rex's virtual package referenced
// (filesystem.FileType, path.Component, and so on). They are reproduced
// here, in Galatea's own package, so the FSAL interface depends on
// nothing outside the standard library. See DEC-005.

// Component of a pathname: a string guaranteed to be a valid Unix
// filename. Transcribed from bb-storage's path.Component, minus the
// Windows validator (Galatea is macOS-only).
type Component struct {
	name string
}

// NewComponent creates a pathname component. It fails when the name is
// empty, ".", "..", or contains a slash or NUL.
func NewComponent(name string) (Component, bool) {
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, "/\x00") {
		return Component{}, false
	}
	return Component{name: name}, true
}

// MustNewComponent is NewComponent that panics on an invalid name.
func MustNewComponent(name string) Component {
	c, ok := NewComponent(name)
	if !ok {
		panic("invalid component name: " + name)
	}
	return c
}

func (c Component) String() string {
	return c.name
}

// FileType enumerates the type of a file. Transcribed from bb-storage's
// filesystem.FileType; the iota order is preserved so values match the
// source we lift the NFSv4 server from later.
type FileType int

const (
	// FileTypeRegularFile means the file is a regular file.
	FileTypeRegularFile FileType = iota
	// FileTypeDirectory means the file is a directory.
	FileTypeDirectory
	// FileTypeSymlink means the file is a symbolic link.
	FileTypeSymlink
	// FileTypeBlockDevice means the file is a block device.
	FileTypeBlockDevice
	// FileTypeCharacterDevice means the file is a character device.
	FileTypeCharacterDevice
	// FileTypeFIFO means the file is a FIFO.
	FileTypeFIFO
	// FileTypeSocket means the file is a socket.
	FileTypeSocket
	// FileTypeOther means the file is of none of the above types.
	FileTypeOther
)

// RegionType distinguishes data from holes for sparse-file seeking.
// Transcribed from bb-storage's filesystem.RegionType; the values (1, 2)
// match SEEK_DATA/SEEK_HOLE conventions in the source.
type RegionType int

const (
	// Data is the start of a region containing data.
	Data RegionType = 1
	// Hole is the start of a region that is a hole.
	Hole RegionType = 2
)

// DeviceNumber identifies a device by major and minor number.
// Transcribed from bb-storage's filesystem.DeviceNumber.
type DeviceNumber struct {
	major uint32
	minor uint32
}

// NewDeviceNumberFromMajorMinor builds a DeviceNumber from its parts.
func NewDeviceNumberFromMajorMinor(major, minor uint32) DeviceNumber {
	return DeviceNumber{major: major, minor: minor}
}

// ToMajorMinor decomposes a DeviceNumber back into major and minor.
func (d DeviceNumber) ToMajorMinor() (uint32, uint32) {
	return d.major, d.minor
}

// FileInfo is the name-and-type pair GetFileInfo returns. Transcribed
// from bb-storage's filesystem.FileInfo (the subset the FSAL uses).
type FileInfo struct {
	name         Component
	fileType     FileType
	isExecutable bool
}

// NewFileInfo constructs a FileInfo.
func NewFileInfo(name Component, fileType FileType, isExecutable bool) FileInfo {
	return FileInfo{name: name, fileType: fileType, isExecutable: isExecutable}
}

// Name returns the component name of the file.
func (fi *FileInfo) Name() Component { return fi.name }

// Type returns the file's type.
func (fi *FileInfo) Type() FileType { return fi.fileType }

// IsExecutable reports whether the file carries the execute permission.
func (fi *FileInfo) IsExecutable() bool { return fi.isExecutable }
