package main

import (
	"context"
	"testing"
	"time"
)

// TestServeGracefulShutdown covers AC6's signal-handling half deterministically:
// it drives the context-cancellation path that SIGINT/SIGTERM trigger in main
// (signal.NotifyContext), without raising a real OS signal (which is flaky in
// tests). doServe must close its listener and return nil — not an error — when
// the context is cancelled.
func TestServeGracefulShutdown(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- doServe(ctx, "", "127.0.0.1:0") }()

	// Let the listener come up and enter Accept, then signal shutdown.
	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("doServe returned %v on ctx-cancel, want nil (graceful shutdown)", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("doServe did not return within 2s of ctx-cancel — shutdown is not graceful")
	}
}
