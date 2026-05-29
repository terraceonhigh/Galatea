// Package virtual defines Galatea's public filesystem abstraction layer
// (FSAL): the interface a host plugs a backend into to obtain a
// Finder-visible NFSv4 mount on macOS.
//
// The interface is hand-cut from Buildbarn's bb-remote-execution
// virtual package (Apache-2.0), reproduced at full fidelity so the
// eventual lift of bb-rex's NFSv4 server stays mechanical, but with the
// handful of bb-storage leaf types it touched (path.Component,
// filesystem.FileType, and friends) replaced by Galatea-native
// equivalents. The result depends on nothing outside the Go standard
// library. See docs/DECISIONS.md, DEC-001 through DEC-005, for the
// reasoning behind the de-coupling.
//
// The three node interfaces — Node, Directory, Leaf — mirror the three
// kinds of object a POSIX filesystem exposes. Every method is prefixed
// Virtual to avoid colliding with the like-named operations a host's own
// filesystem package is likely to define.
package virtual
