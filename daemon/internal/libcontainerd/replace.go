package libcontainerd

import (
	"context"

	containerd "github.com/containerd/containerd/v2/client"
	cerrdefs "github.com/containerd/errdefs"
	"github.com/containerd/log"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"

	"github.com/moby/moby/v2/daemon/internal/libcontainerd/types"
)

// ReplaceContainer creates a new container, replacing any existing container
// with the same id if necessary.
func ReplaceContainer(ctx context.Context, client types.Client, id string, spec *specs.Spec, shim string, runtimeOptions any, opts ...containerd.NewContainerOpts) (types.Container, error) {
	newContainer := func() (types.Container, error) {
		return client.NewContainer(ctx, id, spec, shim, runtimeOptions, opts...)
	}
	ctr, err := newContainer()
	if err == nil || !cerrdefs.IsConflict(err) {
		return ctr, err
	}

	log.G(ctx).WithContext(ctx).WithField("container", id).Debug("A container already exists with the same ID. Attempting to clean up the old container.")
	ctr, err = client.LoadContainer(ctx, id)
	if err != nil {
		if cerrdefs.IsNotFound(err) {
			// Task failed successfully: the container no longer exists,
			// despite us not doing anything. May as well try to create
			// the container again. It might succeed.
			return newContainer()
		}
		return nil, errors.Wrap(err, "could not load stale containerd container object")
	}
	tsk, err := ctr.Task(ctx)
	if err != nil {
		if cerrdefs.IsNotFound(err) {
			goto deleteContainer
		}
		// There is no point in trying to delete the container if we
		// cannot determine whether or not it has a task. The containerd
		// client would just try to load the task itself, get the same
		// error, and give up.
		return nil, errors.Wrap(err, "could not load stale containerd task object")
	}
	if err := tsk.ForceDelete(ctx); err != nil {
		if !cerrdefs.IsNotFound(err) {
			return nil, errors.Wrap(err, "could not delete stale containerd task object")
		}
		// The task might have exited on its own. Proceed with
		// attempting to delete the container.
	}
deleteContainer:
	if err := ctr.Delete(ctx); err != nil && !cerrdefs.IsNotFound(err) {
		return nil, errors.Wrap(err, "could not delete stale containerd container object")
	}

	return newContainer()
}
