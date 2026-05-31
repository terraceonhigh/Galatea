package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	vpath "github.com/terraceonhigh/galatea/internal/bb/filesystem/path"
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

// TestFuseFSLinks is the Phase A1 (structural-ops) gate: symlink / readlink /
// link route through the fuseFS layer to the app's ops. Verified against a real
// passthrough on a host temp dir, the same discipline as the write-path test.
// (chown/utimens/fallocate are NOT covered: the lifted NFSv4.0 server rejects
// owner/time SETATTR at the wire decode and has no ALLOCATE op — they are
// server/protocol-layer ceilings, documented in GOAL-B-libfuse.md, not shim
// wiring. The live gate — a real C tool's full op set — stays Architect-gated.)
func TestFuseFSLinks(t *testing.T) {
	root := t.TempDir()
	fsRoot, _ := NewFuseRoot(passthroughOps(root))
	ctx := context.Background()

	// A target regular file to point at and hard-link.
	createAttrs := &virtual.Attributes{}
	createAttrs.SetPermissions(virtual.PermissionsRead | virtual.PermissionsWrite)
	target, _, _, st := fsRoot.VirtualOpenChild(ctx, virtual.MustNewComponent("f.txt"),
		virtual.ShareMaskWrite, createAttrs, &virtual.OpenExistingOptions{}, 0, &virtual.Attributes{})
	if st != virtual.StatusOK || target == nil {
		t.Fatalf("create f.txt: status=%v", st)
	}
	if _, st := target.VirtualWrite([]byte("body"), 0); st != virtual.StatusOK {
		t.Fatalf("write f.txt: %v", st)
	}

	// symlink: VirtualSymlink(target="f.txt", name="link") → host symlink.
	linkAttrs := &virtual.Attributes{}
	symleaf, _, st := fsRoot.VirtualSymlink(ctx, vpath.UNIXFormat.NewParser("f.txt"),
		virtual.MustNewComponent("link"), virtual.AttributesMaskFileHandle, linkAttrs)
	if st != virtual.StatusOK || symleaf == nil {
		t.Fatalf("symlink: status=%v leaf=%v", st, symleaf)
	}
	if len(linkAttrs.GetFileHandle()) == 0 {
		t.Fatal("symlink: file handle not set on returned attributes (opCreate NF4LNK reads it back)")
	}
	fi, err := os.Lstat(filepath.Join(root, "link"))
	if err != nil || fi.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("symlink effect on host: mode=%v err=%v", fi.Mode(), err)
	}
	if tgt, err := os.Readlink(filepath.Join(root, "link")); err != nil || tgt != "f.txt" {
		t.Fatalf("host readlink: %q err=%v", tgt, err)
	}

	// readlink: GETATTR(SymlinkTarget) on the symlink leaf resolves the target,
	// exactly as the server's opReadLink does.
	var ra virtual.Attributes
	symleaf.VirtualGetAttributes(ctx, virtual.AttributesMaskFileType|virtual.AttributesMaskSymlinkTarget, &ra)
	if ft := ra.GetFileType(); ft != virtual.FileTypeSymlink {
		t.Fatalf("readlink: file type %v (want symlink)", ft)
	}
	parser, ok := ra.GetSymlinkTarget()
	if !ok {
		t.Fatal("readlink: symlink target attribute not set")
	}
	if got, ok := parserToString(parser); !ok || got != "f.txt" {
		t.Fatalf("readlink target: %q ok=%v (want f.txt)", got, ok)
	}

	// Symlink targets are arbitrary text, not always normalizable paths: an
	// absolute target and a multi-component relative one must round-trip VERBATIM
	// through parserToString (pt_symlink stores the target unmodified, so a wrong
	// Readlink here means our Parser→string transform mangled it).
	for i, tgt := range []string{"/abs/elsewhere", "../sibling/x", "a/b/c"} {
		name := virtual.MustNewComponent("ln" + string(rune('0'+i)))
		if _, _, st := fsRoot.VirtualSymlink(ctx, vpath.UNIXFormat.NewParser(tgt),
			name, virtual.AttributesMaskFileHandle, &virtual.Attributes{}); st != virtual.StatusOK {
			t.Fatalf("symlink %q: %v", tgt, st)
		}
		if got, err := os.Readlink(filepath.Join(root, name.String())); err != nil || got != tgt {
			t.Fatalf("symlink target round-trip: stored %q, want %q (err=%v)", got, tgt, err)
		}
	}

	// link: VirtualLink(name="hard", target leaf) → host hard link, nlink == 2.
	if _, st := fsRoot.VirtualLink(ctx, virtual.MustNewComponent("hard"), target, 0, &virtual.Attributes{}); st != virtual.StatusOK {
		t.Fatalf("link: %v", st)
	}
	hi, err := os.Stat(filepath.Join(root, "hard"))
	if err != nil {
		t.Fatalf("link effect: hard missing: %v", err)
	}
	if got, err := os.ReadFile(filepath.Join(root, "hard")); err != nil || string(got) != "body" {
		t.Fatalf("hard link content: %q err=%v", got, err)
	}
	if sys, ok := hi.Sys().(*syscall.Stat_t); ok && sys.Nlink != 2 {
		t.Fatalf("hard link nlink = %d (want 2)", sys.Nlink)
	}

	// A hard link across connections is EXDEV, never silently accepted.
	other := t.TempDir()
	otherRoot, _ := NewFuseRoot(passthroughOps(other))
	if _, st := otherRoot.(*fuseDir).VirtualLink(ctx, virtual.MustNewComponent("x"), target, 0, &virtual.Attributes{}); st != virtual.StatusErrXDev {
		t.Fatalf("cross-conn link: status=%v (want XDev)", st)
	}
}

// TestFuseFSUtimens covers the utimens half of the time-attribute lift:
// VirtualSetAttributes carrying an mtime → op->utimens, with the effect landing
// on the host file and round-tripping back through getattr (st_mtimespec →
// AttributesMaskLastDataModificationTime). atime is UTIME_OMIT (the FSAL has no
// atime). The server-side decode of time_modify_set is proven separately by
// TestConformanceSetattrMtime; this proves the shim translation.
func TestFuseFSUtimens(t *testing.T) {
	root := t.TempDir()
	fsRoot, _ := NewFuseRoot(passthroughOps(root))
	ctx := context.Background()

	createAttrs := &virtual.Attributes{}
	createAttrs.SetPermissions(virtual.PermissionsRead | virtual.PermissionsWrite)
	leaf, _, _, st := fsRoot.VirtualOpenChild(ctx, virtual.MustNewComponent("t.txt"),
		virtual.ShareMaskWrite, createAttrs, &virtual.OpenExistingOptions{}, 0, &virtual.Attributes{})
	if st != virtual.StatusOK || leaf == nil {
		t.Fatalf("create t.txt: %v", st)
	}

	// Set atime and mtime to DISTINCT values; both must land on the host file in
	// their own field (op->utimens carries each, UTIME_OMIT for any not supplied).
	wantM := time.Unix(1700000000, 0) // 2023
	wantA := time.Unix(1600000000, 0) // 2020
	in := &virtual.Attributes{}
	in.SetLastDataModificationTime(wantM)
	in.SetLastAccessTime(wantA)
	if st := leaf.VirtualSetAttributes(ctx, in, 0, &virtual.Attributes{}); st != virtual.StatusOK {
		t.Fatalf("setattr times: %v", st)
	}

	// effect on the host file: mtime and atime both set, independently.
	fi, err := os.Stat(filepath.Join(root, "t.txt"))
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if fi.ModTime().Unix() != wantM.Unix() {
		t.Fatalf("host mtime = %d, want %d", fi.ModTime().Unix(), wantM.Unix())
	}
	if sys, ok := fi.Sys().(*syscall.Stat_t); ok && sys.Atimespec.Sec != wantA.Unix() {
		t.Fatalf("host atime = %d, want %d (mtime must not leak in)", sys.Atimespec.Sec, wantA.Unix())
	}

	// round-trips back through getattr
	var ra virtual.Attributes
	leaf.VirtualGetAttributes(ctx, virtual.AttributesMaskLastDataModificationTime|virtual.AttributesMaskLastAccessTime, &ra)
	if got, ok := ra.GetLastDataModificationTime(); !ok || got.Unix() != wantM.Unix() {
		t.Fatalf("getattr mtime = %d ok=%v, want %d", got.Unix(), ok, wantM.Unix())
	}
	if got, ok := ra.GetLastAccessTime(); !ok || got.Unix() != wantA.Unix() {
		t.Fatalf("getattr atime = %d ok=%v, want %d", got.Unix(), ok, wantA.Unix())
	}
}
