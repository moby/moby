//go:build !no_embedded_containerd

package embedded

import (
	"net"
	"path/filepath"

	"github.com/docker/go-connections/sockets"

	// Linux-specific containerd plugin registrations: the overlayfs
	// snapshotter, the walking differ, the cgroups task monitor (for container
	// metrics), and the runc runtime options type. Cross-platform plugins are
	// registered in server.go.
	_ "github.com/containerd/containerd/api/types/runc/options"
	_ "github.com/containerd/containerd/v2/core/metrics/cgroups"
	_ "github.com/containerd/containerd/v2/plugins/diff/walking/plugin"
	_ "github.com/containerd/containerd/v2/plugins/snapshots/native/plugin"
	_ "github.com/containerd/containerd/v2/plugins/snapshots/overlay/plugin"
)

func defaultAddress(stateDir string) string {
	return filepath.Join(stateDir, "containerd.sock")
}

func listen(address string) (net.Listener, error) {
	return sockets.NewUnixSocketWithOpts(address, sockets.WithChmod(0o660))
}
