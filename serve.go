// Package galatea is the public entry point to Galatea's userspace NFSv4 server.
//
// Galatea lets a host expose any backend that implements the FSAL interface
// (github.com/terraceonhigh/galatea/pkg/virtual: Directory / Leaf) as a volume
// the macOS kernel NFS client can mount — no kernel extension, no root. Serve
// stands the server up over a backend; the host then mounts it with the stock
// client:
//
//	mount_nfs -o vers=4.0,port=N,mountport=N,tcp localhost:/ <mountpoint>
//
// The server (and the wire protocol) live under internal/, which external
// modules cannot import — this package is the supported public surface. A second
// consumer in another repository (e.g. an NTFS backend) imports this package,
// supplies its own virtual.Directory + HandleResolver, and calls Serve.
package galatea

import (
	"context"
	"fmt"
	"net"

	nfssrv "github.com/terraceonhigh/galatea/internal/nfsv4"
	nfsproto "github.com/terraceonhigh/galatea/internal/xdr/pkg/protocols/nfsv4"
	"github.com/terraceonhigh/galatea/internal/xdr/pkg/rpcserver"
	"github.com/terraceonhigh/galatea/pkg/virtual"
)

// Serve runs Galatea's NFSv4 server over root (with resolver mapping NFSv4 file
// handles back to nodes — see virtual.HandleResolver) on the loopback TCP address
// addr, until ctx is cancelled.
//
// It is read-WRITE capable: the backend decides whether mutating operations
// succeed (a read-only FSAL returns ROFS; a writable one carries the writes). The
// underlying program constructor is named NewReadOnlyProgram for historical
// reasons only — writes flow through it (proven against the writable in-memory
// FSAL). So an NTFS or other read-write backend serves correctly here.
//
// Serve blocks. On ctx cancellation it closes the listener, lets in-flight
// connections drain (NFS clients tolerate a server that restarts), and returns
// nil. It returns a non-nil error only if it cannot listen, or on an unexpected
// accept failure that is not a cancellation.
func Serve(ctx context.Context, root virtual.Directory, resolver virtual.HandleResolver, addr string) error {
	program := nfssrv.NewReadOnlyProgram(root, resolver)
	server := rpcserver.NewServer(
		map[uint32]rpcserver.Service{
			nfsproto.NFS4_PROGRAM_PROGRAM_NUMBER: nfsproto.NewNfs4ProgramService(program),
		},
		nfssrv.NewSystemAuthenticator(),
	)

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("galatea: listen %s: %w", addr, err)
	}
	defer ln.Close()

	// Close the listener on cancellation so the blocking Accept below returns;
	// distinguish that expected wake-up from a genuine accept error.
	go func() {
		<-ctx.Done()
		_ = ln.Close()
	}()

	for {
		conn, err := ln.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		go func() {
			_ = server.HandleConnection(conn, conn)
			_ = conn.Close()
		}()
	}
}
