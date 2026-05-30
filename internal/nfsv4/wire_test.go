package nfsv4

import (
	"bytes"
	"encoding/binary"
	"io"
	"net"
	"testing"

	"github.com/terraceonhigh/galatea/internal/xdr/pkg/protocols/nfsv4"
	"github.com/terraceonhigh/galatea/internal/xdr/pkg/protocols/rpcv2"
	"github.com/terraceonhigh/galatea/internal/xdr/pkg/rpcserver"
	"github.com/terraceonhigh/galatea/pkg/virtual"
)

// This is R3's gate: the lifted server answering real ONC-RPC over the wire,
// not just an in-process method call. We stand the server up on one end of a
// net.Pipe via rpcserver.HandleConnection, and hand-build record-marked RPC
// CALLs (with a valid AUTH_SYS credential, which the server authenticates on
// every call) on the other — NULL, then COMPOUND{PUTROOTFH, GETATTR}.

// authsysCred marshals a minimal AUTH_SYS credential body.
func authsysCred(t *testing.T) rpcv2.OpaqueAuth {
	t.Helper()
	var body bytes.Buffer
	parms := &rpcv2.AuthsysParms{Stamp: 0, Machinename: "galatea", Uid: 501, Gid: 20}
	if _, err := parms.WriteTo(&body); err != nil {
		t.Fatalf("marshal AuthsysParms: %v", err)
	}
	return rpcv2.OpaqueAuth{Flavor: rpcv2.AUTH_SYS, Body: body.Bytes()}
}

// encodeCall frames a single-fragment ONC-RPC CALL: a 4-byte record marker
// (length | last-fragment bit) followed by the RpcMsg and any call arguments.
func encodeCall(t *testing.T, xid, proc uint32, cred rpcv2.OpaqueAuth, args io.WriterTo) []byte {
	t.Helper()
	var payload bytes.Buffer
	msg := rpcv2.RpcMsg{
		Xid: xid,
		Body: &rpcv2.RpcMsgBody_CALL{
			Cbody: rpcv2.CallBody{
				Rpcvers: 2,
				Prog:    nfsv4.NFS4_PROGRAM_PROGRAM_NUMBER,
				Vers:    4,
				Proc:    proc,
				Cred:    cred,
				Verf:    rpcv2.OpaqueAuth{Flavor: rpcv2.AUTH_NONE},
			},
		},
	}
	if _, err := msg.WriteTo(&payload); err != nil {
		t.Fatalf("marshal RpcMsg: %v", err)
	}
	if args != nil {
		if _, err := args.WriteTo(&payload); err != nil {
			t.Fatalf("marshal args: %v", err)
		}
	}
	out := make([]byte, 4+payload.Len())
	binary.BigEndian.PutUint32(out, uint32(payload.Len())|0x80000000)
	copy(out[4:], payload.Bytes())
	return out
}

// roundTrip stands up the server on a net.Pipe, sends one framed request, and
// returns the reply payload (record marker stripped). The connection is closed
// after, so the server's HandleConnection loop sees EOF and returns.
func roundTrip(t *testing.T, program nfsv4.Nfs4Program, request []byte) []byte {
	t.Helper()
	serverConn, clientConn := net.Pipe()
	server := rpcserver.NewServer(
		map[uint32]rpcserver.Service{
			nfsv4.NFS4_PROGRAM_PROGRAM_NUMBER: nfsv4.NewNfs4ProgramService(program),
		},
		NewSystemAuthenticator(),
	)
	done := make(chan struct{})
	go func() {
		_ = server.HandleConnection(serverConn, serverConn)
		close(done)
	}()

	if _, err := clientConn.Write(request); err != nil {
		t.Fatalf("write request: %v", err)
	}
	var marker [4]byte
	if _, err := io.ReadFull(clientConn, marker[:]); err != nil {
		t.Fatalf("read record marker: %v", err)
	}
	n := binary.BigEndian.Uint32(marker[:]) &^ 0x80000000
	payload := make([]byte, n)
	if _, err := io.ReadFull(clientConn, payload); err != nil {
		t.Fatalf("read reply payload: %v", err)
	}
	clientConn.Close()
	<-done
	serverConn.Close()
	return payload
}

// decodeAcceptedReply parses a reply payload and asserts it is a MSG_ACCEPTED /
// SUCCESS reply, returning the bytes that follow the RpcMsg (the proc's return
// value, e.g. a Compound4res).
func decodeAcceptedReply(t *testing.T, payload []byte) []byte {
	t.Helper()
	r := bytes.NewReader(payload)
	var msg rpcv2.RpcMsg
	if _, err := msg.ReadFrom(r); err != nil {
		t.Fatalf("decode reply RpcMsg: %v", err)
	}
	reply, ok := msg.Body.(*rpcv2.RpcMsgBody_REPLY)
	if !ok {
		t.Fatalf("reply body type = %T, want *RpcMsgBody_REPLY", msg.Body)
	}
	accepted, ok := reply.Rbody.(*rpcv2.ReplyBody_MSG_ACCEPTED)
	if !ok {
		t.Fatalf("reply rbody type = %T, want *ReplyBody_MSG_ACCEPTED (call was denied?)", reply.Rbody)
	}
	if _, ok := accepted.Areply.ReplyData.(*rpcv2.AcceptedReplyData_SUCCESS); !ok {
		t.Fatalf("accept stat type = %T, want *AcceptedReplyData_SUCCESS", accepted.Areply.ReplyData)
	}
	rest := make([]byte, r.Len())
	_, _ = io.ReadFull(r, rest)
	return rest
}

func newTestProgram() nfsv4.Nfs4Program {
	root := virtual.NewMemoryDirectory(1, virtual.PermissionsRead|virtual.PermissionsExecute, map[string]virtual.Node{
		"hello.txt": virtual.NewMemoryFile(2, virtual.PermissionsRead, []byte("hi")),
	})
	return NewReadOnlyProgram(root, virtual.NewMemoryHandleResolver(root))
}

func TestWireNull(t *testing.T) {
	program := newTestProgram()
	cred := authsysCred(t)
	payload := roundTrip(t, program, encodeCall(t, 1, 0 /* NFSPROC4_NULL */, cred, nil))
	if rest := decodeAcceptedReply(t, payload); len(rest) != 0 {
		t.Errorf("NULL reply carried %d trailing bytes, want 0", len(rest))
	}
}

func TestWireCompound(t *testing.T) {
	program := newTestProgram()
	cred := authsysCred(t)
	args := &nfsv4.Compound4args{
		Tag: "smoke",
		Argarray: []nfsv4.NfsArgop4{
			&nfsv4.NfsArgop4_OP_PUTROOTFH{},
			&nfsv4.NfsArgop4_OP_GETATTR{
				Opgetattr: nfsv4.Getattr4args{AttrRequest: []uint32{1 << nfsv4.FATTR4_TYPE}},
			},
		},
	}
	payload := roundTrip(t, program, encodeCall(t, 2, 1 /* NFSPROC4_COMPOUND */, cred, args))
	rest := decodeAcceptedReply(t, payload)

	var res nfsv4.Compound4res
	if _, err := res.ReadFrom(bytes.NewReader(rest)); err != nil {
		t.Fatalf("decode Compound4res: %v", err)
	}
	if res.Status != nfsv4.NFS4_OK {
		t.Fatalf("COMPOUND status = %v, want NFS4_OK", res.Status)
	}
	if len(res.Resarray) != 2 {
		t.Fatalf("Resarray len = %d, want 2", len(res.Resarray))
	}
}
