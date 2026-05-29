// Package smoke proves the vendored go-xdr codec round-trips inside
// Galatea's module after the import-path rewrite (see ../VENDOR.md). The
// codec's full correctness is upstream's responsibility; this only
// confirms the vendored copy compiles and encodes/decodes here.
package smoke

import (
	"bytes"
	"testing"

	"github.com/terraceonhigh/galatea/internal/xdr/pkg/protocols/nfsv4"
	"github.com/terraceonhigh/galatea/internal/xdr/pkg/runtime"
)

func TestUnsignedHyperRoundTrip(t *testing.T) {
	const want uint64 = 0xDEADBEEFCAFEF00D
	var buf bytes.Buffer
	if _, err := runtime.WriteUnsignedHyper(&buf, want); err != nil {
		t.Fatalf("write: %v", err)
	}
	if buf.Len() != 8 {
		t.Fatalf("encoded length = %d, want 8 (XDR hyper)", buf.Len())
	}
	got, _, err := runtime.ReadUnsignedHyper(&buf)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if got != want {
		t.Fatalf("round-trip: got %#x, want %#x", got, want)
	}
}

func TestNfsFtype4Encoding(t *testing.T) {
	// NfsFtype4 is an NFSv4 enum (a generated type); it encodes as a
	// 4-byte XDR integer. Encode NF4DIR, read the raw int back, confirm.
	var buf bytes.Buffer
	if _, err := nfsv4.NF4DIR.WriteTo(&buf); err != nil {
		t.Fatalf("write: %v", err)
	}
	if buf.Len() != 4 {
		t.Fatalf("encoded length = %d, want 4 (XDR enum)", buf.Len())
	}
	v, _, err := runtime.ReadInt(&buf)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if nfsv4.NfsFtype4(v) != nfsv4.NF4DIR {
		t.Fatalf("round-trip: got %d, want %d (NF4DIR)", v, nfsv4.NF4DIR)
	}
}
