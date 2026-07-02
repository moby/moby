package embedded

import (
	"net"
	"os"
	"path/filepath"

	// Linux-specific containerd plugin registrations: the overlayfs
	// snapshotter, the walking differ, the cgroups task monitor (for container
	// metrics), and the runc runtime options type. Cross-platform plugins are
	// registered in server.go.
	_ "github.com/containerd/containerd/api/types/runc/options"
	_ "github.com/containerd/containerd/v2/core/metrics/cgroups"
	_ "github.com/containerd/containerd/v2/plugins/diff/walking/plugin"
	_ "github.com/containerd/containerd/v2/plugins/snapshots/overlay/plugin"
)

func defaultAddress(stateDir string) string {
	return filepath.Join(stateDir, "containerd.sock")
}

func listen(address string) (net.Listener, error) {
	if err := os.MkdirAll(filepath.Dir(address), 0o700); err != nil {
		return nil, err
	}
	// Remove any stale socket left behind by a previous run.
	if err := os.RemoveAll(address); err != nil {
		return nil, err
	}
	return net.Listen("unix", address)
}
