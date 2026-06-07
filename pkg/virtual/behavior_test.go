package virtual

import (
	"context"
	"testing"
)

// This file characterises the behaviour of the read-only in-memory FSAL
// across the parts of the interface the happy-path tests don't reach:
// seeking, the open-child decision tree, named attributes, and the
// read-only rejection of every mutating operation. It doubles as
// executable documentation of "what works at this version".

func TestSeek(t *testing.T) {
	leaf := NewMemoryFile(1, PermissionsRead, []byte(helloContents))
	size := uint64(len(helloContents))

	if o, st := leaf.VirtualSeek(0, Data); st != StatusOK || o == nil || *o != 0 {
		t.Errorf("Seek(0, Data) = (%v, %v), want (&0, OK)", o, st)
	}
	if o, st := leaf.VirtualSeek(0, Hole); st != StatusOK || o == nil || *o != size {
		t.Errorf("Seek(0, Hole) = (%v, %v), want (&%d, OK)", o, st, size)
	}
	if o, st := leaf.VirtualSeek(size, Data); st != StatusErrNXIO || o != nil {
		t.Errorf("Seek(EOF, Data) = (%v, %v), want (nil, NXIO)", o, st)
	}
	if o, st := leaf.VirtualSeek(3, RegionType(99)); st != StatusErrInval || o != nil {
		t.Errorf("Seek(3, bogus) = (%v, %v), want (nil, Inval)", o, st)
	}
}

func TestOpenChildDecisionTree(t *testing.T) {
	ctx := context.Background()
	read := MustNewComponent("hello.txt")
	dir := MustNewComponent("sub")
	missing := MustNewComponent("nope")

	t.Run("open existing for read", func(t *testing.T) {
		root := buildTree()
		leaf, _, _, st := root.VirtualOpenChild(ctx, read, ShareMaskRead, nil, &OpenExistingOptions{}, 0, &Attributes{})
		if st != StatusOK || leaf == nil {
			t.Fatalf("= (%v, %v), want (leaf, OK)", leaf, st)
		}
	})
	t.Run("missing, no create", func(t *testing.T) {
		root := buildTree()
		_, _, _, st := root.VirtualOpenChild(ctx, missing, ShareMaskRead, nil, &OpenExistingOptions{}, 0, &Attributes{})
		if st != StatusErrNoEnt {
			t.Errorf("= %v, want NoEnt", st)
		}
	})
	t.Run("missing, create requested (read-only rejects)", func(t *testing.T) {
		root := buildTree()
		_, _, _, st := root.VirtualOpenChild(ctx, missing, ShareMaskWrite, &Attributes{}, nil, 0, &Attributes{})
		if st != StatusErrROFS {
			t.Errorf("= %v, want ROFS", st)
		}
	})
	t.Run("exists, create-only", func(t *testing.T) {
		root := buildTree()
		_, _, _, st := root.VirtualOpenChild(ctx, read, ShareMaskRead, nil, nil, 0, &Attributes{})
		if st != StatusErrExist {
			t.Errorf("= %v, want Exist", st)
		}
	})
	t.Run("open a directory as a leaf", func(t *testing.T) {
		root := buildTree()
		_, _, _, st := root.VirtualOpenChild(ctx, dir, ShareMaskRead, nil, &OpenExistingOptions{}, 0, &Attributes{})
		if st != StatusErrIsDir {
			t.Errorf("= %v, want IsDir", st)
		}
	})
	t.Run("write share accepted (files are writable, R6)", func(t *testing.T) {
		root := buildTree()
		_, _, _, st := root.VirtualOpenChild(ctx, read, ShareMaskWrite, nil, &OpenExistingOptions{}, 0, &Attributes{})
		if st != StatusOK {
			t.Errorf("= %v, want OK", st)
		}
	})
}

func TestLookupSubdirectoryIsDirectory(t *testing.T) {
	root := buildTree()
	var attrs Attributes
	child, st := root.VirtualLookup(context.Background(), MustNewComponent("sub"), AttributesMaskFileType, &attrs)
	if st != StatusOK {
		t.Fatalf("lookup(sub) = %v", st)
	}
	d, l := child.GetPair()
	if d == nil || l != nil {
		t.Errorf("GetPair() = (%v, %v), want (directory, nil)", d, l)
	}
	if attrs.GetFileType() != FileTypeDirectory {
		t.Errorf("file type = %v, want FileTypeDirectory", attrs.GetFileType())
	}
}

func TestApplyAndNamedAttributesAreInert(t *testing.T) {
	leaf := NewMemoryFile(1, PermissionsRead, nil)
	if leaf.VirtualApply("anything") {
		t.Error("VirtualApply should report not-intercepted (false)")
	}
	if _, st := leaf.VirtualOpenNamedAttributes(context.Background(), false, 0, &Attributes{}); st != StatusErrNoEnt {
		t.Errorf("VirtualOpenNamedAttributes = %v, want NoEnt", st)
	}
}

func TestEveryDirectoryMutationRejected(t *testing.T) {
	ctx := context.Background()
	root := buildTree()
	name := MustNewComponent("x")
	leaf := NewMemoryFile(99, PermissionsRead, nil)

	if _, st := root.VirtualLink(ctx, name, leaf, 0, &Attributes{}); st != StatusErrROFS {
		t.Errorf("VirtualLink = %v, want ROFS", st)
	}
	if _, _, st := root.VirtualMknod(ctx, name, FileTypeFIFO, 0, &Attributes{}); st != StatusErrROFS {
		t.Errorf("VirtualMknod = %v, want ROFS", st)
	}
	if _, _, st := root.VirtualSymlink(ctx, UNIXFormat.NewParser("/target"), name, 0, &Attributes{}); st != StatusErrROFS {
		t.Errorf("VirtualSymlink = %v, want ROFS", st)
	}
	if _, _, st := root.VirtualRename(MustNewComponent("hello.txt"), root, name); st != StatusErrROFS {
		t.Errorf("VirtualRename = %v, want ROFS", st)
	}
	if st := root.VirtualSetAttributes(ctx, &Attributes{}, 0, &Attributes{}); st != StatusErrROFS {
		t.Errorf("VirtualSetAttributes = %v, want ROFS", st)
	}
}

func TestReadDirResumeFromCookie(t *testing.T) {
	root := buildTree()
	// firstCookie = 1 skips the first sorted entry ("hello.txt").
	var rep collectingReporter
	if st := root.VirtualReadDir(context.Background(), 1, AttributesMaskFileType, &rep); st != StatusOK {
		t.Fatalf("VirtualReadDir = %v", st)
	}
	if len(rep.names) != 1 || rep.names[0] != "sub" {
		t.Errorf("resumed entries = %v, want [sub]", rep.names)
	}
}

// stopEarlyReporter halts enumeration after the first entry.
type stopEarlyReporter struct{ names []string }

func (r *stopEarlyReporter) ReportEntry(_ uint64, name Component, _ DirectoryChild, _ *Attributes) bool {
	r.names = append(r.names, name.String())
	return false
}

func TestReadDirStopsWhenReporterReturnsFalse(t *testing.T) {
	root := buildTree()
	var rep stopEarlyReporter
	if st := root.VirtualReadDir(context.Background(), 0, AttributesMaskFileType, &rep); st != StatusOK {
		t.Fatalf("VirtualReadDir = %v", st)
	}
	if len(rep.names) != 1 {
		t.Errorf("reported %d entries, want 1 (reporter asked to stop)", len(rep.names))
	}
}

func TestComponentValidation(t *testing.T) {
	for _, bad := range []string{"", ".", "..", "a/b", "a\x00b"} {
		if _, ok := NewComponent(bad); ok {
			t.Errorf("NewComponent(%q) accepted, want rejected", bad)
		}
	}
	if _, ok := NewComponent("ordinary.txt"); !ok {
		t.Error("NewComponent(ordinary.txt) rejected, want accepted")
	}
}
