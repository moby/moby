//go:build !no_embedded_containerd

package embedded

import (
	"net"
	"path/filepath"

	"github.com/containerd/containerd/v2/core/metrics/cgroups"
	"github.com/containerd/containerd/v2/plugins"
	"github.com/containerd/plugin"
	"github.com/containerd/ttrpc"
	"github.com/docker/go-connections/sockets"

	// Linux-specific containerd plugin registrations: the overlayfs and native
	// snapshotters, the walking differ, and the runc runtime options type.
	// The cgroups task monitor is registered by the cgroups import above.
	//
	// Cross-platform plugins are registered in server.go.
	_ "github.com/containerd/containerd/api/types/runc/options"
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

func newTTRPCServer() (*ttrpc.Server, error) {
	return ttrpc.NewServer(
		ttrpc.WithServerHandshaker(ttrpc.UnixSocketRequireSameUser()),
	)
}

// disableCgroupsPrometheus disables Prometheus metrics for the cgroups task monitor,
// avoiding process-global collector registration in the embedded server.
func disableCgroupsPrometheus(registration plugin.Registration) any {
	if registration.Type == plugins.TaskMonitorPlugin && registration.ID == "cgroups" {
		if c, ok := registration.Config.(*cgroups.Config); ok {
			cfg := *c
			cfg.NoPrometheus = true
			return &cfg
		}
	}
	return registration.Config
}
