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
	program := NewReadOnlyProgram(root, virtual.NewMemoryHandleResolver(root))
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

// TestHandleResolverRoundTrip exercises DEC-017 Option B's resolver: obtain a
// child's handle via GETFH in one COMPOUND, then replay it via PUTFH in a
// second — the round-trip the macOS client relies on. Without the resolver,
// PUTFH would fail to map the handle back to a node.
func TestHandleResolverRoundTrip(t *testing.T) {
	root := virtual.NewMemoryDirectory(1, virtual.PermissionsRead|virtual.PermissionsExecute, map[string]virtual.Node{
		"hello.txt": virtual.NewMemoryFile(2, virtual.PermissionsRead, []byte("hi")),
	})
	program := NewReadOnlyProgram(root, virtual.NewMemoryHandleResolver(root))
	ctx := context.Background()

	// COMPOUND 1: PUTROOTFH, LOOKUP("hello.txt"), GETFH → the child's handle.
	res1, err := program.NfsV4Nfsproc4Compound(ctx, &nfsv4.Compound4args{
		Argarray: []nfsv4.NfsArgop4{
			&nfsv4.NfsArgop4_OP_PUTROOTFH{},
			&nfsv4.NfsArgop4_OP_LOOKUP{Oplookup: nfsv4.Lookup4args{Objname: "hello.txt"}},
			&nfsv4.NfsArgop4_OP_GETFH{},
		},
	})
	if err != nil {
		t.Fatalf("compound 1: %v", err)
	}
	if res1.Status != nfsv4.NFS4_OK {
		t.Fatalf("compound 1 (PUTROOTFH/LOOKUP/GETFH) status = %v, want NFS4_OK", res1.Status)
	}
	getfh, ok := res1.Resarray[2].(*nfsv4.NfsResop4_OP_GETFH)
	if !ok {
		t.Fatalf("resarray[2] type = %T, want *NfsResop4_OP_GETFH", res1.Resarray[2])
	}
	getfhOK, ok := getfh.Opgetfh.(*nfsv4.Getfh4res_NFS4_OK)
	if !ok {
		t.Fatalf("GETFH result type = %T, want *Getfh4res_NFS4_OK", getfh.Opgetfh)
	}
	handle := getfhOK.Resok4.Object

	// COMPOUND 2: PUTFH(handle), GETATTR → resolves the handle to the node.
	res2, err := program.NfsV4Nfsproc4Compound(ctx, &nfsv4.Compound4args{
		Argarray: []nfsv4.NfsArgop4{
			&nfsv4.NfsArgop4_OP_PUTFH{Opputfh: nfsv4.Putfh4args{Object: handle}},
			&nfsv4.NfsArgop4_OP_GETATTR{
				Opgetattr: nfsv4.Getattr4args{AttrRequest: []uint32{1 << nfsv4.FATTR4_TYPE}},
			},
		},
	})
	if err != nil {
		t.Fatalf("compound 2: %v", err)
	}
	if res2.Status != nfsv4.NFS4_OK {
		t.Fatalf("compound 2 (PUTFH via resolver) status = %v, want NFS4_OK", res2.Status)
	}
}
