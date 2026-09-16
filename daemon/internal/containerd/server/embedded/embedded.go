//go:build !no_embedded_containerd

// Package embedded runs containerd's full gRPC server inside the dockerd
// process.
//
// The same API is served on two endpoints. One is a unix socket (a named pipe
// on Windows) in the daemon's exec-root, used by the plugin executor and by
// tooling such as ctr. The other is an in-memory pipe, used by dockerd's own
// client to avoid socket syscalls.
//
// containerd still runs each container's shim as a separate process, so
// containers keep running across a daemon restart, as they do today.
package embedded
