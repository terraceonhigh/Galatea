package main

import (
	"bytes"
	"testing"

	"github.com/terraceonhigh/galatea/pkg/virtual"
)

// deterministicBytes returns n bytes with a non-repeating-per-256 pattern,
// so a chunk boundary that dropped or duplicated bytes would show up as a
// mismatch rather than passing by luck.
func deterministicBytes(n int) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = byte(i % 251)
	}
	return b
}

// TestStreamLeafMultiChunk covers the path Galatea exists for: reading a
// file larger than the 32 KB stream buffer, across many VirtualRead
// iterations. The happy-path tests and the manual demo only ever touched
// sub-buffer files; this exercises offset advancement and the EOF
// boundary at scale.
func TestStreamLeafMultiChunk(t *testing.T) {
	const bufSize = 32 * 1024
	for _, size := range []int{
		bufSize - 1, // just under one buffer
		bufSize,     // exactly one buffer
		bufSize + 1, // just over: forces a second iteration
		2 * bufSize, // exact multiple: boundary case
		100 * 1024,  // many chunks with a partial tail
	} {
		data := deterministicBytes(size)
		leaf := virtual.NewMemoryFile(1, virtual.PermissionsRead, data)
		var buf bytes.Buffer
		if st := streamLeaf(&buf, leaf); st != virtual.StatusOK {
			t.Fatalf("size=%d: streamLeaf = %s", size, st)
		}
		if buf.Len() != size {
			t.Fatalf("size=%d: streamed %d bytes", size, buf.Len())
		}
		if !bytes.Equal(buf.Bytes(), data) {
			t.Fatalf("size=%d: round-trip mismatch", size)
		}
	}
}

func TestStreamLeafEmpty(t *testing.T) {
	leaf := virtual.NewMemoryFile(1, virtual.PermissionsRead, nil)
	var buf bytes.Buffer
	if st := streamLeaf(&buf, leaf); st != virtual.StatusOK {
		t.Fatalf("streamLeaf(empty) = %s", st)
	}
	if buf.Len() != 0 {
		t.Errorf("streamed %d bytes from empty file", buf.Len())
	}
}
