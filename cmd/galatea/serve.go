package main

import (
	"fmt"
	"net"

	nfssrv "github.com/terraceonhigh/galatea/internal/nfsv4"
	nfsv4 "github.com/terraceonhigh/galatea/internal/xdr/pkg/protocols/nfsv4"
	"github.com/terraceonhigh/galatea/internal/xdr/pkg/rpcserver"
	"github.com/terraceonhigh/galatea/pkg/osfs"
	"github.com/terraceonhigh/galatea/pkg/virtual"
)

// demoTree is a small read-only in-memory FSAL for `galatea serve` to expose.
//
// Why a demo tree and not the osfs backend: serving requires the backend to
// supply file handles + a handle resolver (DEC-017 Option B), which is
// implemented for the in-memory FSAL (pkg/virtual) but not yet for osfs. Once
// osfs grows inode-based handles, `serve` can take a host directory like the
// other subcommands.
func demoTree() virtual.Directory {
	r := virtual.PermissionsRead
	rx := virtual.PermissionsRead | virtual.PermissionsExecute
	return virtual.NewMemoryDirectory(1, rx, map[string]virtual.Node{
		"README.txt": virtual.NewMemoryFile(2, r, []byte("Hello from Galatea — an in-house userspace NFSv4 server.\n")),
		"docs": virtual.NewMemoryDirectory(3, rx, map[string]virtual.Node{
			"note.txt": virtual.NewMemoryFile(4, r, []byte("A second file, one directory deep.\n")),
		}),
	})
}

// doServe stands the lifted NFSv4 server up on a loopback TCP port until
// interrupted. With hostDir empty it serves the in-memory demo tree; otherwise
// it serves that host directory read-only via osfs. This is the R3 → R4 bridge:
// a real socket a macOS NFS client can connect to (proven live — DEC-018).
func doServe(hostDir, addr string) error {
	var root virtual.Directory
	var resolver virtual.HandleResolver
	label := "in-memory demo tree"
	if hostDir == "" {
		root = demoTree()
		resolver = virtual.NewMemoryHandleResolver(root)
	} else {
		r, err := osfs.Root(hostDir)
		if err != nil {
			return fmt.Errorf("open host dir %q: %w", hostDir, err)
		}
		root, resolver, label = r, osfs.NewHandleResolver(r), hostDir
	}
	program := nfssrv.NewReadOnlyProgram(root, resolver)
	server := rpcserver.NewServer(
		map[uint32]rpcserver.Service{
			nfsv4.NFS4_PROGRAM_PROGRAM_NUMBER: nfsv4.NewNfs4ProgramService(program),
		},
		nfssrv.NewSystemAuthenticator(),
	)

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", addr, err)
	}
	defer ln.Close()

	port := ln.Addr().(*net.TCPAddr).Port
	fmt.Printf("galatea: serving %s (read-only) over NFSv4 on %s\n", label, ln.Addr())
	fmt.Printf("galatea: try:  mount_nfs -o vers=4.0,port=%d,mountport=%d,tcp localhost:/ /tmp/galatea-mnt\n", port, port)
	fmt.Printf("galatea: then: ls /tmp/galatea-mnt   (Ctrl-C to stop)\n")

	for {
		conn, err := ln.Accept()
		if err != nil {
			return err
		}
		go func() {
			_ = server.HandleConnection(conn, conn)
			_ = conn.Close()
		}()
	}
}
