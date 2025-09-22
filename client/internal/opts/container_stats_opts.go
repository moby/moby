package opts

import (
	"context"

	"github.com/moby/moby/api/types/container"
	internalshared "github.com/moby/moby/client/internal/shared"
)

type ContainerStatsOptions struct {
	OneShot *container.StatsResponse

	OutputStream chan<- internalshared.StreamItem
}

type ContainerStatsOptionFunc func(ctx context.Context, opts *ContainerStatsOptions) error

func (f ContainerStatsOptionFunc) ApplyContainerStatsOption(ctx context.Context, opts *ContainerStatsOptions) error {
	return f(ctx, opts)
}
