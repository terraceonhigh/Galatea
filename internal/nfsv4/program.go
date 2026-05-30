package nfsv4

import (
	"io"
	"time"

	"github.com/terraceonhigh/galatea/internal/bb/clock"
	"github.com/terraceonhigh/galatea/internal/bb/filesystem/path"
	"github.com/terraceonhigh/galatea/internal/bb/random"
	"github.com/terraceonhigh/galatea/internal/xdr/pkg/protocols/nfsv4"
	"github.com/terraceonhigh/galatea/pkg/virtual"
)

// NewReadOnlyProgram builds an NFSv4.0 server program serving a single FSAL
// backend rooted at root, with defaults suitable for a single-user localhost
// mount. It is Galatea's convenience constructor over bb-rex's NewNFS40Program,
// filling the parameters a host would otherwise have to supply.
//
// The handle resolver is a stub: it rejects every handle. That is sufficient
// for COMPOUNDs that operate on the root or the current filehandle
// (PUTROOTFH/GETATTR/ACCESS), which never resolve an opaque handle, but NOT for
// PUTFH/LOOKUP-driven browsing — those need DEC-017 Option B's real resolver
// (handle → node), still TODO. The root itself must provide a FileHandle
// attribute (the in-memory FSAL does; see pkg/virtual/memory.go), or
// NewNFS40Program panics.
func NewReadOnlyProgram(root virtual.Directory) nfsv4.Nfs4Program {
	stubResolver := virtual.HandleResolver(func(io.ByteReader) (virtual.DirectoryChild, virtual.Status) {
		return virtual.DirectoryChild{}, virtual.StatusErrBadHandle
	})
	var verifier nfsv4.Verifier4
	var stateIDOtherPrefix [stateIDOtherPrefixLength]byte
	return NewNFS40Program(
		root,
		NewOpenedFilesPool(stubResolver),
		random.NewFastSingleThreadedGenerator(),
		verifier,
		stateIDOtherPrefix,
		clock.SystemClock,
		time.Minute, // enforced lease time
		time.Minute, // announced lease time
		path.UNIXFormat,
	)
}
