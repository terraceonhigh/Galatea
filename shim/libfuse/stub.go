//go:build !cgo

// This stub keeps `CGO_ENABLED=0 go build ./...` green. The real libfuse shim
// is cgo-only (it imports "C"), so under CGO_ENABLED=0 its files are excluded;
// without this stub the package would have no buildable Go files and the
// CGO-free build (the AC7 receipt) would fail with "build constraints exclude
// all Go files". The shim itself is never used CGO-disabled — it exists only to
// be built as a c-shared dylib, which requires cgo.
package main

func main() {}
