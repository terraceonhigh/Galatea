package galatea

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/terraceonhigh/galatea/pkg/virtual"
)

func demoRoot() (virtual.Directory, virtual.HandleResolver) {
	root := virtual.NewMemoryDirectory(1, virtual.PermissionsRead|virtual.PermissionsExecute, map[string]virtual.Node{
		"hello.txt": virtual.NewMemoryFile(2, virtual.PermissionsRead, []byte("hi")),
	})
	return root, virtual.NewMemoryHandleResolver(root)
}

// TestServeShutsDownOnCancel covers the public API's lifecycle: Serve blocks
// while serving and returns nil once ctx is cancelled. Driven by cancellation
// (not a raised signal) so it is deterministic.
func TestServeShutsDownOnCancel(t *testing.T) {
	root, resolver := demoRoot()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- Serve(ctx, root, resolver, "127.0.0.1:0") }()

	time.Sleep(50 * time.Millisecond) // let it bind + enter Accept
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Serve returned %v on ctx-cancel, want nil (clean shutdown)", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Serve did not return within 2s of ctx-cancel")
	}
}

// TestServeListenError: a listen failure surfaces as a non-nil error rather than
// hanging. Port 1 is privileged, so binding it as a normal user fails.
func TestServeListenError(t *testing.T) {
	root, resolver := demoRoot()
	if err := Serve(context.Background(), root, resolver, "127.0.0.1:1"); err == nil {
		t.Fatal("Serve on a privileged port returned nil, want a listen error")
	}
}

// TestServeListener covers the bind-then-serve path a host uses when it must
// learn its port before serving: bind 127.0.0.1:0, read l.Addr(), hand the
// listener to ServeListener. Asserts the port is observable before serving, that
// ServeListener returns nil on ctx-cancel, and that it closed the listener it was
// given (the port is free to rebind afterwards).
func TestServeListener(t *testing.T) {
	root, resolver := demoRoot()

	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("bind: %v", err)
	}
	addr := l.Addr().(*net.TCPAddr)
	if addr.Port == 0 {
		t.Fatal("l.Addr() reported port 0 after bind; want the chosen port")
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- ServeListener(ctx, root, resolver, l) }()

	time.Sleep(50 * time.Millisecond) // let it enter Accept
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("ServeListener returned %v on ctx-cancel, want nil", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("ServeListener did not return within 2s of ctx-cancel")
	}

	// ServeListener owns and closes the listener; rebinding the same port proves
	// it was released (and that we don't leak the socket).
	reb, err := net.Listen("tcp", addr.String())
	if err != nil {
		t.Fatalf("rebind %s after ServeListener returned: %v (listener not closed?)", addr, err)
	}
	_ = reb.Close()
}
