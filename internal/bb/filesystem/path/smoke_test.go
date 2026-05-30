package path

import "testing"

// Galatea smoke test for the vendored path package. The package's parsing
// logic is upstream's (bb-storage @ ed02b7a), unchanged; what the Galatea
// lift altered is purely the error construction (grpc status.Error /
// util.StatusWrap → stdlib errors.New / fmt.Errorf — see ../../VENDOR.md).
// So this test exercises one happy path plus the two stripped error paths,
// to prove the strip neither broke resolution nor dropped an error return.

func TestSmokeResolveHappyPath(t *testing.T) {
	// Record a resolved relative path through a Builder.
	b, scopeWalker := EmptyBuilder.Join(NewRelativeScopeWalker(VoidComponentWalker))
	if err := Resolve(UNIXFormat.NewParser("a/b/c"), scopeWalker); err != nil {
		t.Fatalf("Resolve(a/b/c) returned error: %v", err)
	}
	if got, want := b.GetUNIXString(), "a/b/c"; got != want {
		t.Errorf("GetUNIXString() = %q, want %q", got, want)
	}
}

func TestSmokeStrippedErrorPaths(t *testing.T) {
	// unix_format.go: a null byte must still be rejected (errors.New path).
	if err := Resolve(UNIXFormat.NewParser("a\x00b"), NewRelativeScopeWalker(VoidComponentWalker)); err == nil {
		t.Error("Resolve of path with null byte: got nil error, want non-nil")
	}

	// relative_scope_walker.go: an absolute path resolved through a
	// relative-only scope walker must error (errors.New path).
	if err := Resolve(UNIXFormat.NewParser("/abs"), NewRelativeScopeWalker(VoidComponentWalker)); err == nil {
		t.Error("Resolve of absolute path via relative walker: got nil error, want non-nil")
	}
}

func TestSmokeComponent(t *testing.T) {
	if _, ok := NewComponent("valid"); !ok {
		t.Error(`NewComponent("valid") = !ok, want ok`)
	}
	for _, bad := range []string{"", ".", "..", "a/b", "a\x00b"} {
		if _, ok := NewComponent(bad); ok {
			t.Errorf("NewComponent(%q) = ok, want !ok", bad)
		}
	}
}
