package nfsv4

import (
	"context"
	"testing"

	"github.com/terraceonhigh/galatea/internal/xdr/pkg/protocols/nfsv4"
	"github.com/terraceonhigh/galatea/pkg/virtual"
)

// TestSmokeCompound is the first time the lifted NFSv4 server *executes*: it
// builds the server over the in-memory FSAL and drives a NULL call plus a
// COMPOUND { PUTROOTFH; GETATTR } directly (no RPC framing — the cheap path).
// This satisfies the re-scoped R2 behavioural gate (DEC-011/016) and proves the
// dispatch + state-machine + FATTR4 encoding work end to end against a real
// backend.
func TestSmokeCompound(t *testing.T) {
	root := virtual.NewMemoryDirectory(1, virtual.PermissionsRead|virtual.PermissionsExecute, map[string]virtual.Node{
		"hello.txt": virtual.NewMemoryFile(2, virtual.PermissionsRead, []byte("hi")),
	})
	program := NewReadOnlyProgram(root)
	ctx := context.Background()

	if err := program.NfsV4Nfsproc4Null(ctx); err != nil {
		t.Fatalf("NULL: %v", err)
	}

	res, err := program.NfsV4Nfsproc4Compound(ctx, &nfsv4.Compound4args{
		Tag: "smoke",
		Argarray: []nfsv4.NfsArgop4{
			&nfsv4.NfsArgop4_OP_PUTROOTFH{},
			&nfsv4.NfsArgop4_OP_GETATTR{
				Opgetattr: nfsv4.Getattr4args{
					AttrRequest: []uint32{1 << nfsv4.FATTR4_TYPE},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("COMPOUND: %v", err)
	}
	if res.Status != nfsv4.NFS4_OK {
		t.Fatalf("COMPOUND status = %v, want NFS4_OK", res.Status)
	}
	if len(res.Resarray) != 2 {
		t.Fatalf("Resarray len = %d, want 2 (PUTROOTFH, GETATTR)", len(res.Resarray))
	}
}
