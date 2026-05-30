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

// TestMemoryMandatoryAttributes guards against the bite that crashed a live
// macOS mount: the NFSv4 server's FATTR4 encoder panics on any *mandatory*
// attribute the FSAL leaves unset, and the real client requests a broad set.
// (HasNamedAttributes was missing; the server panicked mid-GETATTR.) Assert the
// in-memory FSAL sets every mandatory attribute when asked.
func TestMemoryMandatoryAttributes(t *testing.T) {
	const mandatory = AttributesMaskChangeID | AttributesMaskFileHandle |
		AttributesMaskFileType | AttributesMaskHasNamedAttributes |
		AttributesMaskInodeNumber | AttributesMaskIsInNamedAttributeDirectory |
		AttributesMaskLinkCount
	ctx := context.Background()
	cases := []struct {
		name string
		node Node
	}{
		{"file", NewMemoryFile(2, PermissionsRead, []byte("hi"))},
		{"directory", NewMemoryDirectory(1, PermissionsRead|PermissionsExecute, nil)},
	}
	for _, tc := range cases {
		var a Attributes
		tc.node.VirtualGetAttributes(ctx, mandatory, &a)
		if missing := mandatory &^ a.GetFieldsPresent(); missing != 0 {
			t.Errorf("%s: mandatory attributes left unset: mask %032b", tc.name, missing)
		}
	}
}
