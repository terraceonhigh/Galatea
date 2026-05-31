package galatea

import (
	"context"
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
