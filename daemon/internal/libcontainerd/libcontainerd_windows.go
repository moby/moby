package libcontainerd

import (
	"context"

	containerd "github.com/containerd/containerd/v2/client"
	"github.com/moby/moby/v2/daemon/internal/libcontainerd/local"
	"github.com/moby/moby/v2/daemon/internal/libcontainerd/remote"
	libcontainerdtypes "github.com/moby/moby/v2/daemon/internal/libcontainerd/types"
)

// ContainerdRuntimeEnabled determines whether to use containerd for runtime on Windows.
//
// TODO(thaJeztah): this value is equivalent to checking whether "cli.Config.ContainerdAddr != """ - do we really need it?
var ContainerdRuntimeEnabled = false

// NewClient creates a new libcontainerd client from a containerd client
func NewClient(ctx context.Context, cli *containerd.Client, stateDir, ns string, b libcontainerdtypes.Backend) (libcontainerdtypes.Client, error) {
	if !ContainerdRuntimeEnabled {
		return local.NewClient(ctx, b)
	}
	return remote.NewClient(ctx, cli, stateDir, ns, b)
}
