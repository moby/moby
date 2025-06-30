//go:build !windows

package client

import (
	"context"
	"net"
	"syscall"
)

// DefaultDockerHost defines OS-specific default host if the DOCKER_HOST
// (EnvOverrideHost) environment variable is unset or empty.
const DefaultDockerHost = "unix:///var/run/docker.sock"

// dialPipeContext connects to a Windows named pipe. It is not supported on non-Windows.
func dialPipeContext(_ context.Context, _ string) (net.Conn, error) {
	return nil, syscall.EAFNOSUPPORT
}
