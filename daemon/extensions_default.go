package daemon

import (
	"fmt"
	"maps"
	"path/filepath"
	"slices"
	"strings"

	"github.com/moby/extensions"
	"github.com/moby/extensions/clientpoint"
	"github.com/moby/extensions/serverpoint"
	"github.com/moby/moby/v2/daemon/config"
	"github.com/moby/moby/v2/daemon/internal/jobs"
	containernamegeneratorpb "github.com/moby/moby/v2/extpoints/containernamegenerator/v0/protogen"
	jobspb "github.com/moby/moby/v2/extpoints/jobs/api/v0/protogen"
	servicenamegeneratorpb "github.com/moby/moby/v2/extpoints/servicenamegenerator/v0/protogen"
	namesgeneratorlegacy "github.com/moby/moby/v2/internal/namesgenerator/legacy"
)

// clientProviders lists generated client wiring for points that launched
// extensions may provide. Socket exposure is resolved locally and is not listed.
func clientProviders() []clientpoint.Registration {
	return []clientpoint.Registration{
		containernamegeneratorpb.ClientPoint,
		servicenamegeneratorpb.ClientPoint,
	}
}

// builtinExtensions returns the in-process extensions: the always-on ones,
// plus those built-ins that ship disabled and were opted into through
// enable-extensions. The list is read once at startup; changing it with a
// config reload takes effect on the next daemon start.
func builtinExtensions(cfg *config.Config, d *Daemon) ([]extensions.Extension, error) {
	exts := []extensions.Extension{
		namesgeneratorlegacy.Extension,
		runtimeExtension(d),
	}
	// Built-in extensions that ship with the daemon but stay disabled until
	// opted into by extension ID.
	optional := map[string]func() extensions.Extension{
		jobs.ExtensionID: func() extensions.Extension {
			return jobs.NewExtension(filepath.Join(cfg.Root, "jobs"))
		},
	}
	seen := make(map[string]bool, len(cfg.EnableExtensions))
	for _, id := range cfg.EnableExtensions {
		if seen[id] {
			continue
		}
		seen[id] = true
		build, ok := optional[id]
		if !ok {
			return nil, fmt.Errorf("enable-extensions: unknown built-in extension %q (available: %s)", id, strings.Join(slices.Sorted(maps.Keys(optional)), ", "))
		}
		exts = append(exts, build())
	}
	return exts, nil
}

// pointServers lists the generated server adapters for the points that
// in-process extensions may offer for publication through service.v0. The
// host adapts the typed provider to the socket's gRPC transport with this
// wiring, keeping the extensions themselves transport-agnostic.
func pointServers() []serverpoint.Registration {
	return []serverpoint.Registration{jobspb.ServerPoint}
}
