package osfs

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/terraceonhigh/galatea/pkg/virtual"
)

// makeTree writes a known directory structure into a temp dir and returns
// its path:
//
//	a.txt        "alpha"
//	sub/b.txt    "beta"
func makeTree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("alpha"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "sub", "b.txt"), []byte("beta"), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestRootRejectsNonDirectory(t *testing.T) {
	if _, err := Root(filepath.Join(t.TempDir(), "does-not-exist")); err == nil {
		t.Error("Root on a missing path should error")
	}
	f := filepath.Join(t.TempDir(), "f")
	if err := os.WriteFile(f, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Root(f); err == nil {
		t.Error("Root on a regular file should error")
	}
}

func TestLookupAndReadFile(t *testing.T) {
	root, err := Root(makeTree(t))
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	var attrs virtual.Attributes
	child, st := root.VirtualLookup(ctx, virtual.MustNewComponent("a.txt"),
		virtual.AttributesMaskFileType|virtual.AttributesMaskSizeBytes, &attrs)
	if st != virtual.StatusOK {
		t.Fatalf("lookup a.txt: %s", st)
	}
	if attrs.GetFileType() != virtual.FileTypeRegularFile {
		t.Errorf("type = %v, want regular", attrs.GetFileType())
	}
	if sz, _ := attrs.GetSizeBytes(); sz != 5 {
		t.Errorf("size = %d, want 5", sz)
	}

	_, leaf := child.GetPair()
	if leaf == nil {
		t.Fatal("a.txt is not a leaf")
	}
	buf := make([]byte, 16)
	n, eof, st := leaf.VirtualRead(buf, 0)
	if st != virtual.StatusOK || !eof {
		t.Fatalf("read = (eof=%v, %s)", eof, st)
	}
	if got := string(buf[:n]); got != "alpha" {
		t.Errorf("read %q, want alpha", got)
	}
}

func TestNestedLookup(t *testing.T) {
	root, err := Root(makeTree(t))
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	var a virtual.Attributes
	subChild, st := root.VirtualLookup(ctx, virtual.MustNewComponent("sub"), virtual.AttributesMaskFileType, &a)
	if st != virtual.StatusOK {
		t.Fatalf("lookup sub: %s", st)
	}
	subDir, _ := subChild.GetPair()
	if subDir == nil {
		t.Fatal("sub is not a directory")
	}

	var b virtual.Attributes
	leafChild, st := subDir.VirtualLookup(ctx, virtual.MustNewComponent("b.txt"), virtual.AttributesMaskSizeBytes, &b)
	if st != virtual.StatusOK {
		t.Fatalf("lookup sub/b.txt: %s", st)
	}
	_, leaf := leafChild.GetPair()
	if leaf == nil {
		t.Fatal("b.txt is not a leaf")
	}
	buf := make([]byte, 16)
	n, _, st := leaf.VirtualRead(buf, 0)
	if st != virtual.StatusOK || string(buf[:n]) != "beta" {
		t.Errorf("read sub/b.txt = (%q, %s), want (beta, OK)", string(buf[:n]), st)
	}
}

func TestLookupMissing(t *testing.T) {
	root, err := Root(makeTree(t))
	if err != nil {
		t.Fatal(err)
	}
	var a virtual.Attributes
	_, st := root.VirtualLookup(context.Background(), virtual.MustNewComponent("nope"), virtual.AttributesMaskFileType, &a)
	if st != virtual.StatusErrNoEnt {
		t.Errorf("lookup missing = %s, want NoEnt", st)
	}
}

type nameReporter struct{ names []string }

func (r *nameReporter) ReportEntry(_ uint64, name virtual.Component, _ virtual.DirectoryChild, _ *virtual.Attributes) bool {
	r.names = append(r.names, name.String())
	return true
}

func TestReadDir(t *testing.T) {
	root, err := Root(makeTree(t))
	if err != nil {
		t.Fatal(err)
	}
	var rep nameReporter
	if st := root.VirtualReadDir(context.Background(), 0, virtual.AttributesMaskFileType, &rep); st != virtual.StatusOK {
		t.Fatalf("readdir: %s", st)
	}
	want := []string{"a.txt", "sub"}
	if len(rep.names) != len(want) || rep.names[0] != want[0] || rep.names[1] != want[1] {
		t.Errorf("entries = %v, want %v", rep.names, want)
	}
}

func TestReadPastEOF(t *testing.T) {
	root, err := Root(makeTree(t))
	if err != nil {
		t.Fatal(err)
	}
	var a virtual.Attributes
	child, _ := root.VirtualLookup(context.Background(), virtual.MustNewComponent("a.txt"), 0, &a)
	_, leaf := child.GetPair()
	buf := make([]byte, 8)
	n, eof, st := leaf.VirtualRead(buf, 100)
	if st != virtual.StatusOK || n != 0 || !eof {
		t.Errorf("read past EOF = (n=%d, eof=%v, %s), want (0, true, OK)", n, eof, st)
	}
}

func TestMutationsRejected(t *testing.T) {
	root, err := Root(makeTree(t))
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	name := virtual.MustNewComponent("x")
	if _, _, st := root.VirtualMkdir(name, 0, &virtual.Attributes{}); st != virtual.StatusErrROFS {
		t.Errorf("mkdir = %s, want ROFS", st)
	}
	if _, st := root.VirtualRemove(virtual.MustNewComponent("a.txt"), false, true); st != virtual.StatusErrROFS {
		t.Errorf("remove = %s, want ROFS", st)
	}
	var a virtual.Attributes
	child, _ := root.VirtualLookup(ctx, virtual.MustNewComponent("a.txt"), 0, &a)
	_, leaf := child.GetPair()
	if _, st := leaf.VirtualWrite([]byte("x"), 0); st != virtual.StatusErrROFS {
		t.Errorf("write = %s, want ROFS", st)
	}
}
