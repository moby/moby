package daemon

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/moby/extensions"
	"github.com/moby/extensions/clientpoint"
	"github.com/moby/extensions/host"
	"github.com/moby/extensions/serverpoint"
	"github.com/moby/moby/v2/daemon/config"
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

// optionalBuiltins lists built-in extensions that ship with the daemon but
// stay disabled until opted into through enable-extensions.
var optionalBuiltins = map[string]bool{
	jobs.ExtensionID: true,
}

// daemonExtensions declares extensions that need daemon-owned state.
// Ordinary builtin definitions obtain their bootstrap settings through
// points. The list is unconditional: which optional built-ins actually run
// is the provider policy's decision (see builtinPolicy). enable-extensions
// is validated here so an unknown ID fails startup rather than silently
// enabling nothing. The setting is read once at startup; changing it with a
// config reload takes effect on the next daemon start.
func (daemon *Daemon) daemonExtensions() ([]extensions.Extension, error) {
	cfg := daemon.Config()
	for _, id := range cfg.EnableExtensions {
		if !optionalBuiltins[id] {
			return nil, fmt.Errorf("enable-extensions: unknown built-in extension %q (available: %s)", id, strings.Join(slices.Sorted(maps.Keys(optionalBuiltins)), ", "))
		}
	}
	return []extensions.Extension{
		extensionconfig.New(&cfg),
		runtimeExtension(daemon),
		jobs.NewExtension(daemonReady(daemon)),
	}, nil
}

// builtinPolicy admits every point use except those of optional built-ins
// that were not opted into through enable-extensions. Dropping all of an
// extension's providers makes the host skip it entirely: not initialized,
// no services published. The origin check keeps the gate to built-ins:
// an extension loaded from --extension-dir is enabled by its presence
// there, whatever ID it declares.
func builtinPolicy(cfg *config.Config) host.PointPolicy {
	enabled := make(map[string]bool, len(cfg.EnableExtensions))
	for _, id := range cfg.EnableExtensions {
		enabled[id] = true
	}
	return host.PointPolicyFunc(func(identity extensions.ExtensionIdentity, _ extensions.PointID) host.PointPolicyResult {
		if identity.Origin.Kind == extensions.ExtensionOriginBuiltin && optionalBuiltins[string(identity.ID)] && !enabled[string(identity.ID)] {
			return host.Drop()
		}
		return host.Allow()
	})
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
