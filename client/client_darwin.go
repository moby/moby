//go:build darwin

package client

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"syscall"
)

// defaultDockerHost returns the default Docker socket path for macOS.
// Docker Desktop for Mac uses $HOME/.docker/run/docker.sock
// Reference: https://docs.docker.com/desktop/setup/install/mac-permission-requirements/#installing-symlinks
func defaultDockerHost() string {
	if home, err := os.UserHomeDir(); err == nil {
		sockPath := filepath.Join(home, ".docker/run/docker.sock")
		if _, err := os.Stat(sockPath); err == nil {
			return "unix://" + sockPath
		}
	}
	// Fallback to the legacy path if home directory cannot be determined or socket doesn't exist
	return "unix:///var/run/docker.sock"
}

// DefaultDockerHost defines OS-specific default host if the DOCKER_HOST
// (EnvOverrideHost) environment variable is unset or empty.
var DefaultDockerHost = defaultDockerHost()

// dialPipeContext connects to a Windows named pipe. It is not supported on non-Windows.
func dialPipeContext(_ context.Context, _ string) (net.Conn, error) {
	return nil, syscall.EAFNOSUPPORT
}
