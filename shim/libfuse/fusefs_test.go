package main

import (
	"context"
	"strings"
	"testing"

	"github.com/terraceonhigh/galatea/pkg/virtual"
)

type collectReporter struct{ names []string }

func (r *collectReporter) ReportEntry(_ uint64, name virtual.Component, _ virtual.DirectoryChild, _ *virtual.Attributes) bool {
	r.names = append(r.names, name.String())
	return true
}

// TestFuseFSTranslation is the Phase-1a gate: the fuseFS layer maps the app's
// path-based ops onto virtual.Directory/Leaf correctly, exercised against a stub
// hello tree (fusefs_stub.go). If green, Phase 1b is server + mount glue over
// parts already proven live in R4.
func TestFuseFSTranslation(t *testing.T) {
	root, resolver := NewFuseRoot(stubHelloOps())
	ctx := context.Background()

	// Root is a directory.
	var ra virtual.Attributes
	root.VirtualGetAttributes(ctx, virtual.AttributesMaskFileType, &ra)
	if ra.GetFileType() != virtual.FileTypeDirectory {
		t.Fatalf("root file type = %v, want directory", ra.GetFileType())
	}

	// Lookup of the file: present, regular, 13 bytes.
	var ha virtual.Attributes
	child, st := root.VirtualLookup(ctx, virtual.MustNewComponent("hello"),
		virtual.AttributesMaskFileType|virtual.AttributesMaskSizeBytes, &ha)
	if st != virtual.StatusOK {
		t.Fatalf("lookup hello: status = %v, want OK", st)
	}
	if ha.GetFileType() != virtual.FileTypeRegularFile {
		t.Errorf("hello file type = %v, want regular file", ha.GetFileType())
	}
	if sz, ok := ha.GetSizeBytes(); !ok || sz != 13 {
		t.Errorf("hello size = %d (ok=%v), want 13", sz, ok)
	}

	// Lookup of a missing name → NOENT (errno sign mapped right).
	var na virtual.Attributes
	if _, st := root.VirtualLookup(ctx, virtual.MustNewComponent("nope"), 0, &na); st != virtual.StatusErrNoEnt {
		t.Errorf("lookup of missing name: status = %v, want NoEnt", st)
	}

	// ReadDir of root → exactly ["hello"] (. and .. filtered out).
	rep := &collectReporter{}
	if st := root.VirtualReadDir(ctx, 0, virtual.AttributesMaskFileType, rep); st != virtual.StatusOK {
		t.Fatalf("readdir: status = %v, want OK", st)
	}
	if len(rep.names) != 1 || rep.names[0] != "hello" {
		t.Fatalf("readdir entries = %v, want [hello] (. and .. must be dropped)", rep.names)
	}

	// Read the file's bytes through open + read.
	_, leaf := child.GetPair()
	if leaf == nil {
		t.Fatal("hello did not resolve to a leaf")
	}
	if st := leaf.VirtualOpenSelf(ctx, virtual.ShareMaskRead, &virtual.OpenExistingOptions{}, 0, &virtual.Attributes{}); st != virtual.StatusOK {
		t.Fatalf("open hello: status = %v, want OK", st)
	}
	buf := make([]byte, 64)
	n, _, st := leaf.VirtualRead(buf, 0)
	if st != virtual.StatusOK {
		t.Fatalf("read hello: status = %v, want OK", st)
	}
	if got := string(buf[:n]); got != "Hello World!\n" {
		t.Errorf("read hello = %q, want %q", got, "Hello World!\n")
	}

	// Handle round-trip: a path-based handle resolves back to the node.
	dc, st := resolver(strings.NewReader("/hello"))
	if st != virtual.StatusOK {
		t.Fatalf("resolve /hello handle: status = %v, want OK", st)
	}
	if _, l := dc.GetPair(); l == nil {
		t.Error("resolved /hello handle is not a leaf")
	}
}
