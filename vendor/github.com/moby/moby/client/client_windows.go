package client

import (
	"context"
	"net"

	"github.com/Microsoft/go-winio"
)

// DefaultDockerHost defines OS-specific default host if the DOCKER_HOST
// (EnvOverrideHost) environment variable is unset or empty.
const DefaultDockerHost = "npipe:////./pipe/docker_engine"

// dialPipeContext connects to a Windows named pipe. It is not supported on non-Windows.
func dialPipeContext(ctx context.Context, addr string) (net.Conn, error) {
	return winio.DialPipeContext(ctx, addr)
}
