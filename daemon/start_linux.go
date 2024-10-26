package daemon // import "github.com/docker/docker/daemon"

import (
	"context"

	"github.com/docker/docker/container"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/libcontainerd/types"
	"github.com/docker/docker/oci"
	"github.com/opencontainers/runtime-spec/specs-go"
	"go.opentelemetry.io/otel"
)

// initializeCreatedTask performs any initialization that needs to be done to
// prepare a freshly-created task to be started.
func (daemon *Daemon) initializeCreatedTask(ctx context.Context, tsk types.Task, container *container.Container, spec *specs.Spec) error {
	ctx, span := otel.Tracer("").Start(ctx, "daemon.initializeCreatedTask")
	defer span.End()

	if !container.Config.NetworkDisabled {
		nspath, ok := oci.NamespacePath(spec, specs.NetworkNamespace)
		if ok && nspath == "" { // the runtime has been instructed to create a new network namespace for tsk.
			sb, err := daemon.netController.GetSandbox(container.ID)
			if err != nil {
				return errdefs.System(err)
			}
			return sb.FinishConfig(ctx)
		}
	}
	return nil
}
