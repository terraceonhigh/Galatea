package virtual

import (
	"context"
	"testing"
)

// Compile-time proof that the in-memory types satisfy the interfaces.
var (
	_ Directory = (*memoryDirectory)(nil)
	_ Leaf      = (*memoryFile)(nil)
)

const helloContents = "Hello, Galatea."

// buildTree returns a root directory containing one file (hello.txt) and
// one empty subdirectory (sub).
func buildTree() Directory {
	file := NewMemoryFile(2, PermissionsRead, []byte(helloContents))
	sub := NewMemoryDirectory(3, PermissionsRead|PermissionsExecute, nil)
	return NewMemoryDirectory(1, PermissionsRead|PermissionsExecute, map[string]Node{
		"hello.txt": file.(Node),
		"sub":       sub.(Node),
	})
}

func TestLookupFileAttributes(t *testing.T) {
	root := buildTree()
	ctx := context.Background()

	var attrs Attributes
	child, st := root.VirtualLookup(ctx, MustNewComponent("hello.txt"),
		AttributesMaskFileType|AttributesMaskSizeBytes|AttributesMaskInodeNumber, &attrs)
	if st != StatusOK {
		t.Fatalf("VirtualLookup(hello.txt) = %v, want StatusOK", st)
	}
	if !child.IsSet() {
		t.Fatal("returned child is not set")
	}
	if got := attrs.GetFileType(); got != FileTypeRegularFile {
		t.Errorf("file type = %v, want FileTypeRegularFile", got)
	}
	if got, ok := attrs.GetSizeBytes(); !ok || got != uint64(len(helloContents)) {
		t.Errorf("size = (%d, %v), want (%d, true)", got, ok, len(helloContents))
	}
	if got := attrs.GetInodeNumber(); got != 2 {
		t.Errorf("inode = %d, want 2", got)
	}
}

func TestLookupMissing(t *testing.T) {
	root := buildTree()
	var attrs Attributes
	_, st := root.VirtualLookup(context.Background(), MustNewComponent("nope"), AttributesMaskFileType, &attrs)
	if st != StatusErrNoEnt {
		t.Errorf("VirtualLookup(nope) = %v, want StatusErrNoEnt", st)
	}
}

func TestReadFile(t *testing.T) {
	root := buildTree()
	ctx := context.Background()

	var attrs Attributes
	child, st := root.VirtualLookup(ctx, MustNewComponent("hello.txt"), AttributesMaskFileType, &attrs)
	if st != StatusOK {
		t.Fatalf("lookup: %v", st)
	}
	_, leaf := child.GetPair()
	if leaf == nil {
		t.Fatal("child is not a leaf")
	}

	if st := leaf.VirtualOpenSelf(ctx, ShareMaskRead, &OpenExistingOptions{}, 0, &Attributes{}); st != StatusOK {
		t.Fatalf("VirtualOpenSelf: %v", st)
	}
	buf := make([]byte, 64)
	n, eof, st := leaf.VirtualRead(buf, 0)
	if st != StatusOK {
		t.Fatalf("VirtualRead: %v", st)
	}
	if !eof {
		t.Error("expected eof for a single full read")
	}
	if got := string(buf[:n]); got != helloContents {
		t.Errorf("read %q, want %q", got, helloContents)
	}
	leaf.VirtualClose(ShareMaskRead)
}

func TestReadFileOffset(t *testing.T) {
	leaf := NewMemoryFile(9, PermissionsRead, []byte(helloContents))
	buf := make([]byte, 5)
	n, eof, st := leaf.VirtualRead(buf, 7) // "Galat"
	if st != StatusOK {
		t.Fatalf("VirtualRead: %v", st)
	}
	if eof {
		t.Error("did not expect eof mid-file")
	}
	if got := string(buf[:n]); got != "Galat" {
		t.Errorf("read %q, want %q", got, "Galat")
	}
}

func TestReadAtAndPastEOF(t *testing.T) {
	leaf := NewMemoryFile(9, PermissionsRead, []byte(helloContents))
	size := uint64(len(helloContents))
	buf := make([]byte, 8)
	// At EOF and strictly past it must both report (0, eof) without panic.
	for _, offset := range []uint64{size, size + 1, size + 100} {
		n, eof, st := leaf.VirtualRead(buf, offset)
		if st != StatusOK {
			t.Errorf("VirtualRead(offset=%d) status = %v, want StatusOK", offset, st)
		}
		if n != 0 || !eof {
			t.Errorf("VirtualRead(offset=%d) = (n=%d, eof=%v), want (0, true)", offset, n, eof)
		}
	}
}

type collectingReporter struct {
	names []string
}

func (r *collectingReporter) ReportEntry(nextCookie uint64, name Component, child DirectoryChild, attributes *Attributes) bool {
	r.names = append(r.names, name.String())
	return true
}

func TestReadDirSorted(t *testing.T) {
	root := buildTree()
	var rep collectingReporter
	if st := root.VirtualReadDir(context.Background(), 0, AttributesMaskFileType, &rep); st != StatusOK {
		t.Fatalf("VirtualReadDir: %v", st)
	}
	want := []string{"hello.txt", "sub"}
	if len(rep.names) != len(want) {
		t.Fatalf("entries = %v, want %v", rep.names, want)
	}
	for i := range want {
		if rep.names[i] != want[i] {
			t.Errorf("entry[%d] = %q, want %q", i, rep.names[i], want[i])
		}
	}
}

func TestGetFileInfo(t *testing.T) {
	file := NewMemoryFile(2, PermissionsRead|PermissionsExecute, []byte(helloContents))
	fi := GetFileInfo(MustNewComponent("hello.txt"), file)
	if fi.Name().String() != "hello.txt" {
		t.Errorf("name = %q, want hello.txt", fi.Name().String())
	}
	if fi.Type() != FileTypeRegularFile {
		t.Errorf("type = %v, want FileTypeRegularFile", fi.Type())
	}
	if !fi.IsExecutable() {
		t.Error("expected executable")
	}
}

func TestReadOnlyMutationsRejected(t *testing.T) {
	root := buildTree()
	if _, _, st := root.VirtualMkdir(MustNewComponent("x"), 0, &Attributes{}); st != StatusErrROFS {
		t.Errorf("VirtualMkdir = %v, want StatusErrROFS", st)
	}
	if _, st := root.VirtualRemove(MustNewComponent("hello.txt"), false, true); st != StatusErrROFS {
		t.Errorf("VirtualRemove = %v, want StatusErrROFS", st)
	}
	leaf := NewMemoryFile(2, PermissionsRead, []byte(helloContents))
	if _, st := leaf.VirtualWrite([]byte("x"), 0); st != StatusErrROFS {
		t.Errorf("VirtualWrite = %v, want StatusErrROFS", st)
	}
}
