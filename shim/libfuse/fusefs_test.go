package main

import (
	"context"
	"os"
	"path/filepath"
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

// TestFuseFSWritePath is the Phase-2a gate: the mutating fuseFS methods map onto
// the app's write ops, verified by their *effects* on a host temp dir through a
// read-write passthrough fixture (no .create — exercises the mknod+open path).
func TestFuseFSWritePath(t *testing.T) {
	root := t.TempDir()
	fsRoot, _ := NewFuseRoot(passthroughOps(root))
	ctx := context.Background()

	// mkdir → host dir appears.
	if _, _, st := fsRoot.VirtualMkdir(virtual.MustNewComponent("d"), 0, &virtual.Attributes{}); st != virtual.StatusOK {
		t.Fatalf("mkdir: %v", st)
	}
	if fi, err := os.Stat(filepath.Join(root, "d")); err != nil || !fi.IsDir() {
		t.Fatalf("mkdir effect: err=%v", err)
	}

	// create (mknod+open fallback) → returns a writable leaf.
	createAttrs := &virtual.Attributes{}
	createAttrs.SetPermissions(virtual.PermissionsRead | virtual.PermissionsWrite)
	leaf, _, _, st := fsRoot.VirtualOpenChild(ctx, virtual.MustNewComponent("f.txt"),
		virtual.ShareMaskWrite, createAttrs, &virtual.OpenExistingOptions{}, 0, &virtual.Attributes{})
	if st != virtual.StatusOK || leaf == nil {
		t.Fatalf("create f.txt: status=%v leaf=%v", st, leaf)
	}

	// write → bytes land on the host file, and read back through fuseFS.
	data := []byte("hello write path")
	if n, st := leaf.VirtualWrite(data, 0); st != virtual.StatusOK || n != len(data) {
		t.Fatalf("write: n=%d status=%v", n, st)
	}
	if got, err := os.ReadFile(filepath.Join(root, "f.txt")); err != nil || string(got) != string(data) {
		t.Fatalf("write effect on host: %q err=%v", got, err)
	}
	buf := make([]byte, 64)
	if n, _, st := leaf.VirtualRead(buf, 0); st != virtual.StatusOK || string(buf[:n]) != string(data) {
		t.Fatalf("read back: %q status=%v", buf[:n], st)
	}

	// SETATTR size=0 → truncate (the exact branch the truncate bug lived in).
	in := &virtual.Attributes{}
	in.SetSizeBytes(0)
	if st := leaf.VirtualSetAttributes(ctx, in, 0, &virtual.Attributes{}); st != virtual.StatusOK {
		t.Fatalf("setattr truncate: %v", st)
	}
	if fi, err := os.Stat(filepath.Join(root, "f.txt")); err != nil || fi.Size() != 0 {
		t.Fatalf("truncate effect: size=%d err=%v", fi.Size(), err)
	}

	// SETATTR perms → chmod (read-only: owner write bit clears).
	in2 := &virtual.Attributes{}
	in2.SetPermissions(virtual.PermissionsRead)
	if st := leaf.VirtualSetAttributes(ctx, in2, 0, &virtual.Attributes{}); st != virtual.StatusOK {
		t.Fatalf("setattr chmod: %v", st)
	}
	if fi, err := os.Stat(filepath.Join(root, "f.txt")); err != nil || fi.Mode().Perm()&0o200 != 0 {
		t.Fatalf("chmod effect: mode=%v err=%v (want owner-write cleared)", fi.Mode().Perm(), err)
	}

	// rename f.txt → g.txt.
	if _, _, st := fsRoot.VirtualRename(virtual.MustNewComponent("f.txt"), fsRoot, virtual.MustNewComponent("g.txt")); st != virtual.StatusOK {
		t.Fatalf("rename: %v", st)
	}
	if _, err := os.Stat(filepath.Join(root, "g.txt")); err != nil {
		t.Fatalf("rename effect: g.txt missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "f.txt")); !os.IsNotExist(err) {
		t.Fatalf("rename effect: f.txt should be gone, err=%v", err)
	}

	// remove the file (leaf) and the dir, leaving the root empty.
	if _, st := fsRoot.VirtualRemove(virtual.MustNewComponent("g.txt"), false, true); st != virtual.StatusOK {
		t.Fatalf("remove g.txt: %v", st)
	}
	if _, st := fsRoot.VirtualRemove(virtual.MustNewComponent("d"), true, false); st != virtual.StatusOK {
		t.Fatalf("rmdir d: %v", st)
	}
	if entries, _ := os.ReadDir(root); len(entries) != 0 {
		t.Fatalf("after removes, host dir not empty: %v", entries)
	}
}
