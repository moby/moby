//go:build !no_embedded_containerd

package embedded

import (
	"bytes"
	"net"
	"os/exec"
	"path/filepath"
	"slices"

	"github.com/containerd/containerd/v2/core/metrics/cgroups"
	"github.com/containerd/containerd/v2/plugins"
	"github.com/containerd/containerd/v2/plugins/services"
	"github.com/containerd/plugin"
	"github.com/containerd/ttrpc"
	"github.com/docker/go-connections/sockets"

	// Linux-specific containerd plugin registrations: the overlayfs, native,
	// and EROFS snapshotters, the walking and EROFS differs, and the runc runtime
	// options type.
	// The cgroups task monitor is registered by the cgroups import above.
	//
	// Cross-platform plugins are registered in server.go.
	_ "github.com/containerd/containerd/api/types/runc/options"
	_ "github.com/containerd/containerd/v2/plugins/diff/erofs/plugin"
	_ "github.com/containerd/containerd/v2/plugins/diff/walking/plugin"
	_ "github.com/containerd/containerd/v2/plugins/snapshots/erofs/plugin"
	_ "github.com/containerd/containerd/v2/plugins/snapshots/native/plugin"
	_ "github.com/containerd/containerd/v2/plugins/snapshots/overlay/plugin"
)

// configureErofsDiffer adds the EROFS differ after the walking differ when
// mkfs.erofs supports tar mode. The walking differ returns ErrNotImplemented
// for native EROFS layers, allowing the diff service to fall back to EROFS.
func configureErofsDiffer(cfg *serverConfig) {
	const erofsDiffer = "io.containerd.differ.v1.erofs"
	if slices.Contains(cfg.containerd.DisabledPlugins, erofsDiffer) {
		return
	}

	pluginID := plugins.ServicePlugin.String() + "." + services.DiffService
	pluginConfig, ok := cfg.containerd.Plugins[pluginID].(map[string]any)
	if cfg.containerd.Plugins[pluginID] != nil && !ok {
		return
	}
	if _, configured := pluginConfig["default"]; configured {
		return
	}

	output, err := exec.Command("mkfs.erofs", "--help").CombinedOutput()
	if err != nil || !bytes.Contains(output, []byte("--tar=")) {
		return
	}

	if cfg.containerd.Plugins == nil {
		cfg.containerd.Plugins = make(map[string]any)
	}
	if pluginConfig == nil {
		pluginConfig = make(map[string]any)
	}
	pluginConfig["default"] = []string{"walking", "erofs"}
	cfg.containerd.Plugins[pluginID] = pluginConfig
}

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
