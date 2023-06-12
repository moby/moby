package rootless // import "github.com/docker/docker/pkg/rootless"

import "os"

// RootlessKitDockerProxyBinary is the binary name of rootlesskit-docker-proxy
const RootlessKitDockerProxyBinary = "rootlesskit-docker-proxy"

// RunningWithRootlessKit returns true if running under RootlessKit namespaces.
func RunningWithRootlessKit() bool {
	return os.Getenv("ROOTLESSKIT_STATE_DIR") != ""
}
