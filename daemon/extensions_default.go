package daemon

import (
	"context"
	"fmt"
	"maps"
	"path/filepath"
	"slices"
	"strings"

	"github.com/moby/extensions"
	"github.com/moby/extensions/clientpoint"
	"github.com/moby/extensions/serverpoint"
	"github.com/moby/moby/v2/daemon/internal/extensionconfig"
	"github.com/moby/moby/v2/daemon/internal/extensionstorage/bbolt"
	"github.com/moby/moby/v2/daemon/internal/jobs"

	containernamegeneratorpb "github.com/moby/moby/v2/extpoints/containernamegenerator/v0/protogen"
	daemonconfigpb "github.com/moby/moby/v2/extpoints/daemonconfig/v0/protogen"
	jobspb "github.com/moby/moby/v2/extpoints/jobs/api/v0/protogen"
	servicenamegeneratorpb "github.com/moby/moby/v2/extpoints/servicenamegenerator/v0/protogen"
	storagepb "github.com/moby/moby/v2/extpoints/storage/kv/v0/protogen"
	namesgeneratorlegacy "github.com/moby/moby/v2/internal/namesgenerator/legacy"
)

// builtinExtensions lists in-tree extensions that should be available in the
// engine by default.
var builtinExtensions = []extensions.Extension{
	namesgeneratorlegacy.Extension,
	bbolt.Extension,
}

// daemonExtensions declares extensions that need daemon-owned state: the
// always-on ones, plus those built-ins that ship disabled and were opted into
// through enable-extensions. Ordinary builtin definitions obtain their
// bootstrap settings through points. The list is read once at startup;
// changing it with a config reload takes effect on the next daemon start.
func (daemon *Daemon) daemonExtensions() ([]extensions.Extension, error) {
	cfg := daemon.Config()
	exts := []extensions.Extension{
		extensionconfig.New(&cfg),
		runtimeExtension(daemon),
	}
	// Built-in extensions that ship with the daemon but stay disabled until
	// opted into by extension ID.
	optional := map[string]func() extensions.Extension{
		jobs.ExtensionID: func() extensions.Extension {
			return jobs.NewExtension(filepath.Join(cfg.Root, "jobs"), daemonReady(daemon))
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

// daemonReady reports when the daemon's container backend can serve: the
// extension host is built before container restore completes. Injected into
// extensions that must wait for it, until the framework offers a general
// readiness mechanism.
func daemonReady(d *Daemon) func(context.Context) error {
	return func(ctx context.Context) error {
		select {
		case <-d.startupDone:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// pointServers lists the generated server adapters for the points that
// in-process extensions may offer for publication through service.v0. The
// host adapts the typed provider to the socket's gRPC transport with this
// wiring, keeping the extensions themselves transport-agnostic.
func pointServers() []serverpoint.Registration {
	return []serverpoint.Registration{jobspb.ServerPoint}
}

// clientProviders lists generated client wiring for points that launched
// extensions may provide.
//
// A point that's simply exposed as a service by an extension DOES NOT need to
// be in this list.
// This list only contains points that the daemon calls directly.
func clientProviders() []clientpoint.Registration {
	return []clientpoint.Registration{
		containernamegeneratorpb.ClientPoint,
		servicenamegeneratorpb.ClientPoint,
	}
}

// dependencyProviders lists points that external extensions may call back
// into.
// Points not in this list will not be able to used as dependencies.
func dependencyProviders() []serverpoint.Registration {
	return []serverpoint.Registration{
		storagepb.ServerPoint,
		daemonconfigpb.ServerPoint,
	}
}
