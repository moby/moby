package rootless // import "github.com/docker/docker/pkg/rootless"

import (
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"github.com/rootless-containers/rootlesskit/pkg/api/client"
)

// RootlessKitDockerProxyBinary is the binary name of rootlesskit-docker-proxy
const RootlessKitDockerProxyBinary = "rootlesskit-docker-proxy"

// RunningWithRootlessKit returns true if running under RootlessKit namespaces.
func RunningWithRootlessKit() bool {
	return os.Getenv("ROOTLESSKIT_STATE_DIR") != ""
}

// GetRootlessKitClient returns RootlessKit client
func GetRootlessKitClient() (client.Client, error) {
	stateDir := os.Getenv("ROOTLESSKIT_STATE_DIR")
	if stateDir == "" {
		return nil, errors.New("environment variable `ROOTLESSKIT_STATE_DIR` is not set")
	}
	apiSock := filepath.Join(stateDir, "api.sock")
	return client.New(apiSock)
}
