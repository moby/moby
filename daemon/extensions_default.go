package daemon

import (
	"context"
	"path/filepath"

	"github.com/moby/extensions"
	"github.com/moby/extensions/clientpoint"
	"github.com/moby/extensions/serverpoint"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/v2/daemon/config"
	"github.com/moby/moby/v2/daemon/internal/extensionconfig"
	"github.com/moby/moby/v2/daemon/internal/extensionstorage/bbolt"
	"github.com/moby/moby/v2/daemon/internal/jobs"

	containernamegeneratorpb "github.com/moby/moby/v2/extpoints/containernamegenerator/v0/protogen"
	daemonconfigpb "github.com/moby/moby/v2/extpoints/daemonconfig/v0/protogen"
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

// daemonExtensions declares extensions that need daemon-owned state.
// Ordinary builtin definitions obtain their bootstrap settings through points.
func (daemon *Daemon) daemonExtensions() []extensions.Extension {
	cfg := daemon.Config()
	exts := []extensions.Extension{
		extensionconfig.New(&cfg),
		runtimeExtension(daemon),
	}
	if daemon.jobsExtension != nil {
		exts = append(exts, daemon.jobsExtension)
	}
	return exts
}

// jobsExtension builds the builtin jobs extension when the jobs feature is
// enabled. The daemon holds on to it: unlike other builtins it needs the
// daemon's container backend, which is not usable until container restore
// completes, so the daemon activates it explicitly later in startup. The
// flag is read once at startup; flipping it with a config reload takes
// effect on the next daemon start, like containerd-snapshotter.
func jobsExtension(cfg *config.Config) *jobs.Extension {
	if !cfg.Features["jobs"] {
		return nil
	}
	return jobs.NewExtension(filepath.Join(cfg.Root, "jobs"))
}

// jobsBackend adapts the daemon to the jobs backend interface. Every method
// is served by the daemon directly except ContainerWait, whose channel
// element must be converted: channels are invariant, so the daemon's
// concrete StateStatus channel cannot satisfy the interface-typed one.
type jobsBackend struct {
	*Daemon
}

func (b jobsBackend) ContainerWait(ctx context.Context, name string, condition container.WaitCondition) (<-chan jobs.StateStatus, error) {
	waitC, err := b.Daemon.ContainerWait(ctx, name, condition)
	if err != nil {
		return nil, err
	}
	out := make(chan jobs.StateStatus, 1)
	// The forwarder lives until the container's final exit delivers the
	// status (the daemon-side channel always sends exactly one); under
	// live-restore that can be the rest of the container's life, which is
	// also how long the daemon-side wait it forwards from lives.
	go func() {
		defer close(out)
		if state, ok := <-waitC; ok {
			out <- state
		}
	}()
	return out, nil
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
