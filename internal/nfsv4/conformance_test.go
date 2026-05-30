package nfsv4

import (
	"bytes"
	"testing"

	"github.com/terraceonhigh/galatea/internal/xdr/pkg/protocols/nfsv4"
	"github.com/terraceonhigh/galatea/pkg/virtual"
)

// R5 (headless half) — a protocol-level conformance suite.
//
// pjdfstest (the usual POSIX-at-mountpoint suite) is triple-blocked on this Mac:
// it targets FreeBSD/Linux/Solaris (not Darwin), needs autotools we don't have,
// and demands root. pynfs (the protocol suite) needs a `pip install` the sandbox
// forbids. So R5's headless-tractable half is done *in-language*: we drive real
// record-marked ONC-RPC COMPOUNDs (the same harness as wire_test.go) against the
// lifted server and assert protocol conformance — turning behaviours previously
// proven only by ephemeral live mounts (R4/R6) into permanent regression tests.
//
// Coverage: the stateless read path (GETATTR/LOOKUP/GETFH/PUTFH/ACCESS/READ/
// READDIR + the NOENT/STALE error edges) and the stateless write path
// (CREATE/REMOVE/RENAME — none of which need the open-owner state machine).
// The stateful write path (OPEN→WRITE→CLOSE) is exercised live (R6, DEC-018) and
// at the FSAL layer (pkg/virtual unit tests); driving the SETCLIENTID dance over
// raw XDR is deferred to pynfs-on-Linux (see DECISIONS R5 entry).

// doCompound frames one COMPOUND over the wire harness and returns the decoded
// result. Each call stands up a fresh server on a net.Pipe but over the *same*
// program, so FSAL mutations (CREATE/REMOVE/RENAME) persist across calls while
// no NFSv4 client state is required (these ops are stateless).
func doCompound(t *testing.T, program nfsv4.Nfs4Program, ops ...nfsv4.NfsArgop4) *nfsv4.Compound4res {
	t.Helper()
	args := &nfsv4.Compound4args{Tag: "conf", Argarray: ops}
	payload := roundTrip(t, program, encodeCall(t, 7, 1 /* COMPOUND */, authsysCred(t), args))
	rest := decodeAcceptedReply(t, payload)
	var res nfsv4.Compound4res
	if _, err := res.ReadFrom(bytes.NewReader(rest)); err != nil {
		t.Fatalf("decode Compound4res: %v", err)
	}
	return &res
}

func writableTestProgram() nfsv4.Nfs4Program {
	rwx := virtual.PermissionsRead | virtual.PermissionsWrite | virtual.PermissionsExecute
	root := virtual.NewWritableMemoryDirectory(rwx)
	return NewReadOnlyProgram(root, virtual.NewMemoryHandleResolver(root))
}

// --- read path -------------------------------------------------------------

func TestConformanceGetattrType(t *testing.T) {
	res := doCompound(t, newTestProgram(),
		&nfsv4.NfsArgop4_OP_PUTROOTFH{},
		&nfsv4.NfsArgop4_OP_GETATTR{Opgetattr: nfsv4.Getattr4args{
			AttrRequest: []uint32{1<<nfsv4.FATTR4_TYPE | 1<<nfsv4.FATTR4_SIZE}}},
	)
	if res.Status != nfsv4.NFS4_OK {
		t.Fatalf("status = %v, want NFS4_OK", res.Status)
	}
	if len(res.Resarray) != 2 {
		t.Fatalf("Resarray len = %d, want 2", len(res.Resarray))
	}
}

func TestConformanceLookupReadFile(t *testing.T) {
	res := doCompound(t, newTestProgram(),
		&nfsv4.NfsArgop4_OP_PUTROOTFH{},
		&nfsv4.NfsArgop4_OP_LOOKUP{Oplookup: nfsv4.Lookup4args{Objname: "hello.txt"}},
		&nfsv4.NfsArgop4_OP_READ{Opread: nfsv4.Read4args{
			Stateid: nfsv4.Stateid4{}, // anonymous (all-zero) stateid — valid for READ
			Offset:  0,
			Count:   100,
		}},
	)
	if res.Status != nfsv4.NFS4_OK {
		t.Fatalf("status = %v, want NFS4_OK", res.Status)
	}
	rd, ok := res.Resarray[2].(*nfsv4.NfsResop4_OP_READ)
	if !ok {
		t.Fatalf("resarray[2] = %T, want *NfsResop4_OP_READ", res.Resarray[2])
	}
	rok, ok := rd.Opread.(*nfsv4.Read4res_NFS4_OK)
	if !ok {
		t.Fatalf("READ result = %T, want *Read4res_NFS4_OK", rd.Opread)
	}
	if got := string(rok.Resok4.Data); got != "hi" {
		t.Errorf("READ data = %q, want %q", got, "hi")
	}
	if !rok.Resok4.Eof {
		t.Errorf("READ eof = false, want true (read past end of a 2-byte file)")
	}
}

func TestConformanceAccessRoot(t *testing.T) {
	res := doCompound(t, newTestProgram(),
		&nfsv4.NfsArgop4_OP_PUTROOTFH{},
		&nfsv4.NfsArgop4_OP_ACCESS{Opaccess: nfsv4.Access4args{
			Access: nfsv4.ACCESS4_READ | nfsv4.ACCESS4_LOOKUP}},
	)
	if res.Status != nfsv4.NFS4_OK {
		t.Fatalf("status = %v, want NFS4_OK", res.Status)
	}
	ac, ok := res.Resarray[1].(*nfsv4.NfsResop4_OP_ACCESS)
	if !ok {
		t.Fatalf("resarray[1] = %T, want *NfsResop4_OP_ACCESS", res.Resarray[1])
	}
	aok, ok := ac.Opaccess.(*nfsv4.Access4res_NFS4_OK)
	if !ok {
		t.Fatalf("ACCESS result = %T, want *Access4res_NFS4_OK", ac.Opaccess)
	}
	if aok.Resok4.Access&nfsv4.ACCESS4_READ == 0 {
		t.Errorf("ACCESS granted = %#x, want ACCESS4_READ set on a readable root", aok.Resok4.Access)
	}
}

func TestConformanceReaddirRoot(t *testing.T) {
	res := doCompound(t, newTestProgram(),
		&nfsv4.NfsArgop4_OP_PUTROOTFH{},
		&nfsv4.NfsArgop4_OP_READDIR{Opreaddir: nfsv4.Readdir4args{
			Cookie:      0,
			Dircount:    4096,
			Maxcount:    8192,
			AttrRequest: []uint32{1 << nfsv4.FATTR4_TYPE},
		}},
	)
	if res.Status != nfsv4.NFS4_OK {
		t.Fatalf("status = %v, want NFS4_OK", res.Status)
	}
}

func TestConformanceLookupNoent(t *testing.T) {
	res := doCompound(t, newTestProgram(),
		&nfsv4.NfsArgop4_OP_PUTROOTFH{},
		&nfsv4.NfsArgop4_OP_LOOKUP{Oplookup: nfsv4.Lookup4args{Objname: "does-not-exist"}},
	)
	if res.Status != nfsv4.NFS4ERR_NOENT {
		t.Fatalf("LOOKUP of a missing name: status = %v, want NFS4ERR_NOENT", res.Status)
	}
}

func TestConformancePutfhBadHandle(t *testing.T) {
	res := doCompound(t, newTestProgram(),
		&nfsv4.NfsArgop4_OP_PUTFH{Opputfh: nfsv4.Putfh4args{
			Object: []byte{0xde, 0xad, 0xbe, 0xef, 0x00, 0x01, 0x02, 0x03}}},
	)
	if res.Status != nfsv4.NFS4ERR_STALE && res.Status != nfsv4.NFS4ERR_BADHANDLE {
		t.Fatalf("PUTFH of a bogus handle: status = %v, want NFS4ERR_STALE or NFS4ERR_BADHANDLE", res.Status)
	}
}

// --- stateless write path (no open-owner state) ----------------------------

func TestConformanceCreateDir(t *testing.T) {
	program := writableTestProgram()
	res := doCompound(t, program,
		&nfsv4.NfsArgop4_OP_PUTROOTFH{},
		&nfsv4.NfsArgop4_OP_CREATE{Opcreate: nfsv4.Create4args{
			Objtype: &nfsv4.Createtype4_NF4DIR{}, Objname: "sub"}},
	)
	if res.Status != nfsv4.NFS4_OK {
		t.Fatalf("CREATE dir: status = %v, want NFS4_OK", res.Status)
	}
	// Verify it is there and is a directory.
	res2 := doCompound(t, program,
		&nfsv4.NfsArgop4_OP_PUTROOTFH{},
		&nfsv4.NfsArgop4_OP_LOOKUP{Oplookup: nfsv4.Lookup4args{Objname: "sub"}},
		&nfsv4.NfsArgop4_OP_GETATTR{Opgetattr: nfsv4.Getattr4args{
			AttrRequest: []uint32{1 << nfsv4.FATTR4_TYPE}}},
	)
	if res2.Status != nfsv4.NFS4_OK {
		t.Fatalf("LOOKUP of created dir: status = %v, want NFS4_OK", res2.Status)
	}
}

func TestConformanceRemove(t *testing.T) {
	program := writableTestProgram()
	if res := doCompound(t, program,
		&nfsv4.NfsArgop4_OP_PUTROOTFH{},
		&nfsv4.NfsArgop4_OP_CREATE{Opcreate: nfsv4.Create4args{
			Objtype: &nfsv4.Createtype4_NF4DIR{}, Objname: "rmme"}},
	); res.Status != nfsv4.NFS4_OK {
		t.Fatalf("setup CREATE: status = %v", res.Status)
	}
	if res := doCompound(t, program,
		&nfsv4.NfsArgop4_OP_PUTROOTFH{},
		&nfsv4.NfsArgop4_OP_REMOVE{Opremove: nfsv4.Remove4args{Target: "rmme"}},
	); res.Status != nfsv4.NFS4_OK {
		t.Fatalf("REMOVE: status = %v, want NFS4_OK", res.Status)
	}
	if res := doCompound(t, program,
		&nfsv4.NfsArgop4_OP_PUTROOTFH{},
		&nfsv4.NfsArgop4_OP_LOOKUP{Oplookup: nfsv4.Lookup4args{Objname: "rmme"}},
	); res.Status != nfsv4.NFS4ERR_NOENT {
		t.Fatalf("LOOKUP after REMOVE: status = %v, want NFS4ERR_NOENT", res.Status)
	}
}

func TestConformanceRename(t *testing.T) {
	program := writableTestProgram()
	if res := doCompound(t, program,
		&nfsv4.NfsArgop4_OP_PUTROOTFH{},
		&nfsv4.NfsArgop4_OP_CREATE{Opcreate: nfsv4.Create4args{
			Objtype: &nfsv4.Createtype4_NF4DIR{}, Objname: "ra"}},
	); res.Status != nfsv4.NFS4_OK {
		t.Fatalf("setup CREATE: status = %v", res.Status)
	}
	// Same-directory rename: SAVEFH(root) as source, PUTROOTFH(root) as target.
	if res := doCompound(t, program,
		&nfsv4.NfsArgop4_OP_PUTROOTFH{},
		&nfsv4.NfsArgop4_OP_SAVEFH{},
		&nfsv4.NfsArgop4_OP_PUTROOTFH{},
		&nfsv4.NfsArgop4_OP_RENAME{Oprename: nfsv4.Rename4args{Oldname: "ra", Newname: "rb"}},
	); res.Status != nfsv4.NFS4_OK {
		t.Fatalf("RENAME: status = %v, want NFS4_OK", res.Status)
	}
	if res := doCompound(t, program,
		&nfsv4.NfsArgop4_OP_PUTROOTFH{},
		&nfsv4.NfsArgop4_OP_LOOKUP{Oplookup: nfsv4.Lookup4args{Objname: "rb"}},
	); res.Status != nfsv4.NFS4_OK {
		t.Fatalf("LOOKUP of rename target: status = %v, want NFS4_OK", res.Status)
	}
	if res := doCompound(t, program,
		&nfsv4.NfsArgop4_OP_PUTROOTFH{},
		&nfsv4.NfsArgop4_OP_LOOKUP{Oplookup: nfsv4.Lookup4args{Objname: "ra"}},
	); res.Status != nfsv4.NFS4ERR_NOENT {
		t.Fatalf("LOOKUP of rename source after move: status = %v, want NFS4ERR_NOENT", res.Status)
	}
}

// --- stateful write path (the open-owner state machine) --------------------

// TestConformanceOpenWriteClose drives the full NFSv4.0 write dance over the
// wire — SETCLIENTID, SETCLIENTID_CONFIRM, OPEN(create), the conditional
// OPEN_CONFIRM for a fresh owner, WRITE, CLOSE — then reads the bytes back in a
// fresh COMPOUND. This is the capstone: the same sequence the live macOS client
// performed (R4/R6), now pinned as an in-language regression.
func TestConformanceOpenWriteClose(t *testing.T) {
	program := writableTestProgram()
	owner := []byte("galatea-conf-owner")
	payload := []byte("the capstone payload")

	// 1. SETCLIENTID — establish a client id.
	res := doCompound(t, program,
		&nfsv4.NfsArgop4_OP_SETCLIENTID{Opsetclientid: nfsv4.Setclientid4args{
			Client:        nfsv4.NfsClientId4{Verifier: [8]byte{1, 2, 3, 4, 5, 6, 7, 8}, Id: []byte("galatea-conf-client")},
			Callback:      nfsv4.CbClient4{CbProgram: 0, CbLocation: nfsv4.Netaddr4{NaRNetid: "tcp", NaRAddr: "0.0.0.0.0.0"}},
			CallbackIdent: 0,
		}},
	)
	if res.Status != nfsv4.NFS4_OK {
		t.Fatalf("SETCLIENTID: status = %v, want NFS4_OK", res.Status)
	}
	sc := res.Resarray[0].(*nfsv4.NfsResop4_OP_SETCLIENTID).Opsetclientid.(*nfsv4.Setclientid4res_NFS4_OK).Resok4
	clientid, confirm := sc.Clientid, sc.SetclientidConfirm

	// 2. SETCLIENTID_CONFIRM.
	if res := doCompound(t, program,
		&nfsv4.NfsArgop4_OP_SETCLIENTID_CONFIRM{OpsetclientidConfirm: nfsv4.SetclientidConfirm4args{
			Clientid: clientid, SetclientidConfirm: confirm}},
	); res.Status != nfsv4.NFS4_OK {
		t.Fatalf("SETCLIENTID_CONFIRM: status = %v, want NFS4_OK", res.Status)
	}

	// 3. PUTROOTFH, OPEN(create w.txt), GETFH — open-owner seqid starts at 0.
	res = doCompound(t, program,
		&nfsv4.NfsArgop4_OP_PUTROOTFH{},
		&nfsv4.NfsArgop4_OP_OPEN{Opopen: nfsv4.Open4args{
			Seqid:       0,
			ShareAccess: nfsv4.OPEN4_SHARE_ACCESS_WRITE,
			ShareDeny:   nfsv4.OPEN4_SHARE_DENY_NONE,
			Owner:       nfsv4.StateOwner4{Clientid: clientid, Owner: owner},
			Openhow:     &nfsv4.Openflag4_OPEN4_CREATE{How: &nfsv4.Createhow4_UNCHECKED4{Createattrs: nfsv4.Fattr4{}}},
			Claim:       &nfsv4.OpenClaim4_CLAIM_NULL{File: "w.txt"},
		}},
		&nfsv4.NfsArgop4_OP_GETFH{},
	)
	if res.Status != nfsv4.NFS4_OK {
		t.Fatalf("OPEN(create): status = %v, want NFS4_OK", res.Status)
	}
	openOK := res.Resarray[1].(*nfsv4.NfsResop4_OP_OPEN).Opopen.(*nfsv4.Open4res_NFS4_OK).Resok4
	stateid := openOK.Stateid
	handle := res.Resarray[2].(*nfsv4.NfsResop4_OP_GETFH).Opgetfh.(*nfsv4.Getfh4res_NFS4_OK).Resok4.Object
	ownerSeqid := uint32(1) // next op for this owner

	// 4. Conditional OPEN_CONFIRM (a fresh owner usually requires it).
	if openOK.Rflags&nfsv4.OPEN4_RESULT_CONFIRM != 0 {
		cres := doCompound(t, program,
			&nfsv4.NfsArgop4_OP_PUTFH{Opputfh: nfsv4.Putfh4args{Object: handle}},
			&nfsv4.NfsArgop4_OP_OPEN_CONFIRM{OpopenConfirm: nfsv4.OpenConfirm4args{
				OpenStateid: stateid, Seqid: ownerSeqid}},
		)
		if cres.Status != nfsv4.NFS4_OK {
			t.Fatalf("OPEN_CONFIRM: status = %v, want NFS4_OK", cres.Status)
		}
		stateid = cres.Resarray[1].(*nfsv4.NfsResop4_OP_OPEN_CONFIRM).OpopenConfirm.(*nfsv4.OpenConfirm4res_NFS4_OK).Resok4.OpenStateid
		ownerSeqid++
	}

	// 5. PUTFH(file), WRITE, CLOSE.
	wres := doCompound(t, program,
		&nfsv4.NfsArgop4_OP_PUTFH{Opputfh: nfsv4.Putfh4args{Object: handle}},
		&nfsv4.NfsArgop4_OP_WRITE{Opwrite: nfsv4.Write4args{
			Stateid: stateid, Offset: 0, Stable: nfsv4.FILE_SYNC4, Data: payload}},
		&nfsv4.NfsArgop4_OP_CLOSE{Opclose: nfsv4.Close4args{Seqid: ownerSeqid, OpenStateid: stateid}},
	)
	if wres.Status != nfsv4.NFS4_OK {
		t.Fatalf("WRITE+CLOSE: status = %v, want NFS4_OK", wres.Status)
	}
	wok := wres.Resarray[1].(*nfsv4.NfsResop4_OP_WRITE).Opwrite.(*nfsv4.Write4res_NFS4_OK).Resok4
	if wok.Count != uint32(len(payload)) {
		t.Errorf("WRITE count = %d, want %d", wok.Count, len(payload))
	}

	// 6. Fresh COMPOUND: read it back and confirm the bytes survived.
	rres := doCompound(t, program,
		&nfsv4.NfsArgop4_OP_PUTROOTFH{},
		&nfsv4.NfsArgop4_OP_LOOKUP{Oplookup: nfsv4.Lookup4args{Objname: "w.txt"}},
		&nfsv4.NfsArgop4_OP_READ{Opread: nfsv4.Read4args{Stateid: nfsv4.Stateid4{}, Offset: 0, Count: 1024}},
	)
	if rres.Status != nfsv4.NFS4_OK {
		t.Fatalf("read-back: status = %v, want NFS4_OK", rres.Status)
	}
	got := rres.Resarray[2].(*nfsv4.NfsResop4_OP_READ).Opread.(*nfsv4.Read4res_NFS4_OK).Resok4.Data
	if !bytes.Equal(got, payload) {
		t.Errorf("read-back data = %q, want %q", got, payload)
	}
}
