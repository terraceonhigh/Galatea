// Command galatea is a CLI that navigates a host directory through
// Galatea's filesystem abstraction layer (FSAL). Every operation it
// performs — lookup, readdir, read, getattr — goes through the
// virtual.Directory/virtual.Leaf interface, exactly as a future NFSv4
// server would. It stands in for the NFS client a real mount will
// provide, and lets the FSAL be exercised end-to-end from the command
// line with no privileges. See docs/DECISIONS.md DEC-006.
package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/terraceonhigh/galatea/pkg/osfs"
	"github.com/terraceonhigh/galatea/pkg/virtual"
)

const usage = `galatea — navigate a directory through Galatea's FSAL

usage:
  galatea ls   <host-dir> [path]   list a directory
  galatea stat <host-dir> [path]   show a node's attributes
  galatea cat  <host-dir> <path>   print a file's contents
  galatea tree <host-dir> [path]   print the tree recursively

<host-dir> is the directory Galatea exposes as the FSAL root.
[path] is a slash-separated path *within* the FSAL (default: the root).
`

func main() {
	if len(os.Args) < 3 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	cmd, hostDir := os.Args[1], os.Args[2]
	sub := ""
	if len(os.Args) > 3 {
		sub = os.Args[3]
	}

	root, err := osfs.Root(hostDir)
	if err != nil {
		fatalf("cannot open host dir %q: %v", hostDir, err)
	}

	switch cmd {
	case "ls":
		err = doLs(root, sub)
	case "stat":
		err = doStat(root, sub)
	case "cat":
		if sub == "" {
			fatalf("cat requires a path within the FSAL")
		}
		err = doCat(root, sub)
	case "tree":
		err = doTree(root, sub)
	default:
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	if err != nil {
		fatalf("%v", err)
	}
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "galatea: "+format+"\n", args...)
	os.Exit(1)
}

// splitPath splits an FSAL path into its non-empty components.
func splitPath(p string) []string {
	parts := []string{}
	for _, s := range strings.Split(p, "/") {
		if s != "" {
			parts = append(parts, s)
		}
	}
	return parts
}

// resolve walks the FSAL from root to the node named by sub. It returns
// the resolved node as a directory or a leaf (exactly one is non-nil),
// having traversed only through VirtualLookup.
func resolve(root virtual.Directory, sub string) (virtual.Directory, virtual.Leaf, error) {
	parts := splitPath(sub)
	if len(parts) == 0 {
		return root, nil, nil
	}
	ctx := context.Background()
	cur := root
	for i, p := range parts {
		comp, ok := virtual.NewComponent(p)
		if !ok {
			return nil, nil, fmt.Errorf("invalid path component %q", p)
		}
		var attrs virtual.Attributes
		child, st := cur.VirtualLookup(ctx, comp, virtual.AttributesMaskFileType, &attrs)
		if st != virtual.StatusOK {
			return nil, nil, fmt.Errorf("lookup %q: %s", p, st)
		}
		dir, leaf := child.GetPair()
		if i == len(parts)-1 {
			return dir, leaf, nil
		}
		if dir == nil {
			return nil, nil, fmt.Errorf("%q is not a directory", p)
		}
		cur = dir
	}
	return cur, nil, nil
}

const lsMask = virtual.AttributesMaskFileType | virtual.AttributesMaskPermissions | virtual.AttributesMaskSizeBytes

// typeChar returns a single-character type indicator.
func typeChar(ft virtual.FileType) byte {
	switch ft {
	case virtual.FileTypeDirectory:
		return 'd'
	case virtual.FileTypeSymlink:
		return 'l'
	case virtual.FileTypeFIFO:
		return 'p'
	case virtual.FileTypeSocket:
		return 's'
	case virtual.FileTypeCharacterDevice:
		return 'c'
	case virtual.FileTypeBlockDevice:
		return 'b'
	default:
		return '-'
	}
}

// permString renders Galatea's collapsed rwx permissions.
func permString(p virtual.Permissions) string {
	b := []byte("---")
	if p&virtual.PermissionsRead != 0 {
		b[0] = 'r'
	}
	if p&virtual.PermissionsWrite != 0 {
		b[1] = 'w'
	}
	if p&virtual.PermissionsExecute != 0 {
		b[2] = 'x'
	}
	return string(b)
}

func formatRow(name string, a *virtual.Attributes) string {
	ft := virtual.FileTypeOther
	if a.GetFieldsPresent()&virtual.AttributesMaskFileType != 0 {
		ft = a.GetFileType()
	}
	perms, _ := a.GetPermissions()
	size, hasSize := a.GetSizeBytes()
	sizeStr := "-"
	if hasSize && ft != virtual.FileTypeDirectory {
		sizeStr = fmt.Sprintf("%d", size)
	}
	return fmt.Sprintf("%c%s %10s  %s", typeChar(ft), permString(perms), sizeStr, name)
}

// lsReporter collects directory entries from VirtualReadDir.
type lsReporter struct {
	rows []string
}

func (r *lsReporter) ReportEntry(_ uint64, name virtual.Component, _ virtual.DirectoryChild, attributes *virtual.Attributes) bool {
	r.rows = append(r.rows, formatRow(name.String(), attributes))
	return true
}

func doLs(root virtual.Directory, sub string) error {
	dir, leaf, err := resolve(root, sub)
	if err != nil {
		return err
	}
	if leaf != nil {
		// ls of a single file: show its row.
		var a virtual.Attributes
		leaf.VirtualGetAttributes(context.Background(), lsMask, &a)
		fmt.Println(formatRow(lastComponent(sub), &a))
		return nil
	}
	var rep lsReporter
	if st := dir.VirtualReadDir(context.Background(), 0, lsMask, &rep); st != virtual.StatusOK {
		return fmt.Errorf("readdir: %s", st)
	}
	for _, row := range rep.rows {
		fmt.Println(row)
	}
	return nil
}

func doStat(root virtual.Directory, sub string) error {
	dir, leaf, err := resolve(root, sub)
	if err != nil {
		return err
	}
	var node virtual.Node = dir
	if leaf != nil {
		node = leaf
	}
	mask := virtual.AttributesMaskFileType | virtual.AttributesMaskPermissions |
		virtual.AttributesMaskSizeBytes | virtual.AttributesMaskInodeNumber |
		virtual.AttributesMaskLinkCount | virtual.AttributesMaskLastDataModificationTime |
		virtual.AttributesMaskChangeID
	var a virtual.Attributes
	node.VirtualGetAttributes(context.Background(), mask, &a)

	name := sub
	if name == "" {
		name = "(root)"
	}
	fmt.Printf("path:    %s\n", name)
	if a.GetFieldsPresent()&virtual.AttributesMaskFileType != 0 {
		fmt.Printf("type:    %s\n", fileTypeName(a.GetFileType()))
	}
	if p, ok := a.GetPermissions(); ok {
		fmt.Printf("perms:   %s\n", permString(p))
	}
	if sz, ok := a.GetSizeBytes(); ok {
		fmt.Printf("size:    %d bytes\n", sz)
	}
	if a.GetFieldsPresent()&virtual.AttributesMaskInodeNumber != 0 {
		fmt.Printf("inode:   %d\n", a.GetInodeNumber())
	}
	if a.GetFieldsPresent()&virtual.AttributesMaskLinkCount != 0 {
		fmt.Printf("nlink:   %d\n", a.GetLinkCount())
	}
	if t, ok := a.GetLastDataModificationTime(); ok {
		fmt.Printf("mtime:   %s\n", t.Format("2006-01-02 15:04:05"))
	}
	return nil
}

func doCat(root virtual.Directory, sub string) error {
	_, leaf, err := resolve(root, sub)
	if err != nil {
		return err
	}
	if leaf == nil {
		return fmt.Errorf("%q is a directory", sub)
	}
	ctx := context.Background()
	if st := leaf.VirtualOpenSelf(ctx, virtual.ShareMaskRead, &virtual.OpenExistingOptions{}, 0, &virtual.Attributes{}); st != virtual.StatusOK {
		return fmt.Errorf("open: %s", st)
	}
	defer leaf.VirtualClose(virtual.ShareMaskRead)

	buf := make([]byte, 32*1024)
	var offset uint64
	for {
		n, eof, st := leaf.VirtualRead(buf, offset)
		if st != virtual.StatusOK {
			return fmt.Errorf("read at %d: %s", offset, st)
		}
		if n > 0 {
			if _, werr := os.Stdout.Write(buf[:n]); werr != nil {
				return werr
			}
			offset += uint64(n)
		}
		if eof || n == 0 {
			break
		}
	}
	return nil
}

func doTree(root virtual.Directory, sub string) error {
	dir, leaf, err := resolve(root, sub)
	if err != nil {
		return err
	}
	label := sub
	if label == "" {
		label = "."
	}
	if leaf != nil {
		fmt.Println(label)
		return nil
	}
	fmt.Println(label)
	return treeInto(dir, "")
}

func treeInto(dir virtual.Directory, prefix string) error {
	c := &treeReporter{}
	if st := dir.VirtualReadDir(context.Background(), 0, virtual.AttributesMaskFileType, c); st != virtual.StatusOK {
		return fmt.Errorf("readdir: %s", st)
	}
	for i, name := range c.names {
		last := i == len(c.names)-1
		branch, nextPrefix := "├── ", prefix+"│   "
		if last {
			branch, nextPrefix = "└── ", prefix+"    "
		}
		fmt.Printf("%s%s%s\n", prefix, branch, name)
		if c.dirs[i] != nil {
			if err := treeInto(c.dirs[i], nextPrefix); err != nil {
				return err
			}
		}
	}
	return nil
}

type treeReporter struct {
	names []string
	dirs  []virtual.Directory // nil entry == a leaf
}

func (r *treeReporter) ReportEntry(_ uint64, name virtual.Component, child virtual.DirectoryChild, attributes *virtual.Attributes) bool {
	dir, _ := child.GetPair()
	r.names = append(r.names, name.String())
	r.dirs = append(r.dirs, dir)
	return true
}

func lastComponent(p string) string {
	parts := splitPath(p)
	if len(parts) == 0 {
		return "."
	}
	return parts[len(parts)-1]
}

func fileTypeName(ft virtual.FileType) string {
	switch ft {
	case virtual.FileTypeRegularFile:
		return "regular file"
	case virtual.FileTypeDirectory:
		return "directory"
	case virtual.FileTypeSymlink:
		return "symbolic link"
	case virtual.FileTypeFIFO:
		return "FIFO"
	case virtual.FileTypeSocket:
		return "socket"
	case virtual.FileTypeCharacterDevice:
		return "character device"
	case virtual.FileTypeBlockDevice:
		return "block device"
	default:
		return "other"
	}
}
