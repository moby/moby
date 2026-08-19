package daemon

import (
	"context"
	"path/filepath"

	"github.com/moby/extensions"
	"github.com/moby/extensions/clientpoint"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/v2/daemon/config"
	"github.com/moby/moby/v2/daemon/internal/jobs"
)

// clientProviders lists generated client wiring for points that launched
// extensions may provide. Socket exposure is resolved locally and is not listed.
func clientProviders() []clientpoint.Registration {
	return nil
}

// builtinExtensions returns the in-process extensions selected by daemon config.
func builtinExtensions(*config.Config) []extensions.Extension {
	return nil
}

// jobsExtension builds the builtin jobs extension when the jobs feature is
// enabled. It is kept out of builtinExtensions because the daemon must hold
// on to it: the extension is activated separately, once the container
// backend is ready. The flag is read once at startup; flipping it with a
// config reload takes effect on the next daemon start, like
// containerd-snapshotter.
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
