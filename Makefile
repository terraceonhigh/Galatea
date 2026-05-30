# Galatea — an in-house userspace NFSv4 server for macOS.
# Plain Go module; these targets are conveniences over `go ...`.

.PHONY: build vet test test-race test-conformance fmt all

all: build vet test

build:
	go build ./...

vet:
	go vet ./...

test:
	go test ./...

test-race:
	go test -race ./...

fmt:
	gofmt -l -w .

# R5 conformance gate. The protocol-level conformance suite drives real
# record-marked ONC-RPC COMPOUNDs against the lifted NFSv4 server (read path,
# stateless write CREATE/REMOVE/RENAME, and the full stateful OPEN→WRITE→CLOSE
# dance). It is in-language and headless — no external mount, root, or suite.
#
# Why not pjdfstest/pynfs here: pjdfstest targets FreeBSD/Linux/Solaris (not
# Darwin), needs autotools, and demands root; pynfs needs a pip install the
# sandbox forbids. Both are deferred to a Linux NFS client / CI (see
# docs/DECISIONS.md, the R5 entry). The POSIX-at-mountpoint and protocol suites
# they provide are complementary to — not a replacement for — this suite.
test-conformance:
	go test -race -v -run TestConformance ./internal/nfsv4/
