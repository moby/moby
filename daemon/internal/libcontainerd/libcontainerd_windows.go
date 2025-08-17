package libcontainerd

import (
	"context"

	containerd "github.com/containerd/containerd/v2/client"

	"github.com/moby/moby/v2/daemon/internal/libcontainerd/local"
	"github.com/moby/moby/v2/daemon/internal/libcontainerd/remote"
	libcontainerdtypes "github.com/moby/moby/v2/daemon/internal/libcontainerd/types"
)

// NewClient creates a new libcontainerd client from a containerd client
func NewClient(ctx context.Context, cli *containerd.Client, stateDir, ns string, b libcontainerdtypes.Backend) (libcontainerdtypes.Client, error) {
	if cli == nil {
		return local.NewClient(ctx, b)
	}
	return remote.NewClient(ctx, cli, stateDir, ns, b)
}
