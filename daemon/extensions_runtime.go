package daemon

import (
	"context"

	"github.com/moby/extensions"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/v2/daemon/server/backend"
	runtimev0 "github.com/moby/moby/v2/extpoints/runtime/v0"
)

// runtimeExtensionID identifies the builtin extension through which the
// daemon provides the container runtime point.
const runtimeExtensionID = "org.mobyproject.runtime.v1"

// runtimeExtension declares the builtin extension providing the container
// runtime point, backed by the daemon.
func runtimeExtension(d *Daemon) extensions.Extension {
	return extensions.New(extensions.Declaration{
		ID:        runtimeExtensionID,
		Providers: []extensions.Provider{runtimev0.Point.Provide(daemonRuntime{d: d})},
	})
}

// daemonRuntime provides the container runtime point from the daemon. Every
// method is delegated directly except ContainerWait, whose channel element
// must be converted: channels are invariant, so the daemon's concrete
// StateStatus channel cannot satisfy the interface-typed one.
type daemonRuntime struct {
	d *Daemon
}

// Ready blocks until container restore has completed: the extension host is
// built early in daemon construction, before the container backend can serve
// requests.
func (r daemonRuntime) Ready(ctx context.Context) error {
	select {
	case <-r.d.startupDone:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (r daemonRuntime) ContainerCreate(ctx context.Context, config backend.ContainerCreateConfig) (container.CreateResponse, error) {
	return r.d.ContainerCreate(ctx, config)
}

func (r daemonRuntime) ContainerStart(ctx context.Context, name string, checkpoint string, checkpointDir string) error {
	return r.d.ContainerStart(ctx, name, checkpoint, checkpointDir)
}

func (r daemonRuntime) ContainerStop(ctx context.Context, name string, options backend.ContainerStopOptions) error {
	return r.d.ContainerStop(ctx, name, options)
}

func (r daemonRuntime) ContainerRm(name string, config *backend.ContainerRmConfig) error {
	return r.d.ContainerRm(name, config)
}

func (r daemonRuntime) ContainerWait(ctx context.Context, name string, condition container.WaitCondition) (<-chan runtimev0.StateStatus, error) {
	waitC, err := r.d.ContainerWait(ctx, name, condition)
	if err != nil {
		return nil, err
	}
	out := make(chan runtimev0.StateStatus, 1)
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
