package daemon

import (
	"path/filepath"

	"github.com/moby/extensions"
	"github.com/moby/extensions/clientpoint"
	"github.com/moby/extensions/serverpoint"
	"github.com/moby/moby/v2/daemon/config"
	"github.com/moby/moby/v2/daemon/internal/jobs"
	jobspb "github.com/moby/moby/v2/extpoints/jobs/api/v0/protogen"
)

// clientProviders lists generated client wiring for points that launched
// extensions may provide. Socket exposure is resolved locally and is not listed.
func clientProviders() []clientpoint.Registration {
	return nil
}

// builtinExtensions returns the in-process extensions selected by daemon
// config, plus the runtime extension through which the daemon provides
// container operations. Feature flags are read once at startup; flipping one
// with a config reload takes effect on the next daemon start, like
// containerd-snapshotter.
func builtinExtensions(cfg *config.Config, d *Daemon) []extensions.Extension {
	exts := []extensions.Extension{runtimeExtension(d)}
	if cfg.Features["jobs"] {
		exts = append(exts, jobs.NewExtension(filepath.Join(cfg.Root, "jobs")))
	}
	return exts
}

// pointServers lists the generated server adapters for the points that
// in-process extensions may offer for publication through service.v0. The
// host adapts the typed provider to the socket's gRPC transport with this
// wiring, keeping the extensions themselves transport-agnostic.
func pointServers() []serverpoint.Registration {
	return []serverpoint.Registration{jobspb.ServerPoint}
}
