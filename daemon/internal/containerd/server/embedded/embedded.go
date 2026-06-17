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

import (
	"context"
	"net"
	"time"
)

// Config holds parameters for the embedded containerd daemon.
type Config struct {
	// DisabledPlugins lists containerd plugin URIs to disable (e.g. CRI).
	DisabledPlugins []string
	// LogLevel overrides the containerd log level (default: inherit from dockerd).
	LogLevel string
}

// DaemonOpt configures the embedded daemon.
type DaemonOpt func(*Config)

// WithCRIDisabled disables the CRI plugin, which would otherwise start a
// Kubernetes-facing gRPC server.
func WithCRIDisabled() DaemonOpt {
	return func(c *Config) {
		c.DisabledPlugins = append(c.DisabledPlugins, "io.containerd.grpc.v1.cri")
	}
}

// WithLogLevel sets the containerd log level for the embedded daemon.
func WithLogLevel(level string) DaemonOpt {
	return func(c *Config) { c.LogLevel = level }
}

// Daemon is an in-process containerd server.
type Daemon interface {
	// Address returns the containerd gRPC address (unix socket path, or named
	// pipe on Windows) external clients and tooling should dial.
	Address() string
	// Dial returns an in-memory connection to the server, which dockerd's own
	// client uses to avoid socket syscalls. The signature matches
	// grpc.WithContextDialer, and the addr argument is ignored.
	Dial(ctx context.Context, addr string) (net.Conn, error)
	// WaitTimeout waits up to d for the server to stop after Shutdown.
	WaitTimeout(d time.Duration) error
	// Shutdown gracefully stops the in-process server and waits for it to exit.
	Shutdown(ctx context.Context) error
}
