package virtual

import (
	"context"
	"testing"
)

// TestMemoryFileHandle covers DEC-017 Option B step 1: the in-memory FSAL
// self-assigns a stable file handle from its inode, which the NFSv4 server
// reads at construction (and would panic without). See memory.go.
func TestMemoryFileHandle(t *testing.T) {
	ctx := context.Background()

	f := NewMemoryFile(42, PermissionsRead, []byte("hi"))
	var fa Attributes
	f.VirtualGetAttributes(ctx, AttributesMaskFileHandle, &fa)
	got := fa.GetFileHandle()
	if len(got) != 8 {
		t.Fatalf("file handle length = %d, want 8", len(got))
	}
	if want := memoryFileHandle(42); string(got) != string(want) {
		t.Errorf("file handle = %x, want %x", got, want)
	}

	// A node with a different inode must get a different handle.
	d := NewMemoryDirectory(7, PermissionsRead|PermissionsExecute, nil)
	var da Attributes
	d.VirtualGetAttributes(ctx, AttributesMaskFileHandle, &da)
	if string(da.GetFileHandle()) == string(got) {
		t.Error("nodes with distinct inodes produced the same file handle")
	}
}
