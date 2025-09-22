package containerstats

import (
	"context"
	"errors"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client/internal/opts"
	internalshared "github.com/moby/moby/client/internal/shared"
)

type Option interface {
	ApplyContainerStatsOption(ctx context.Context, opts *opts.ContainerStatsOptions) error
}

type StreamItem = internalshared.StreamItem

func WithStream(out chan<- StreamItem) Option {
	return opts.ContainerStatsOptionFunc(func(ctx context.Context, o *opts.ContainerStatsOptions) error {
		if o.OutputStream != nil {
			return errors.New("only single output stream can be set")
		}
		if o.OneShot != nil {
			return errors.New("can't stream with oneshot")
		}
		o.OutputStream = out
		return nil
	})
}

func WithOneshot(out *container.StatsResponse) Option {
	return opts.ContainerStatsOptionFunc(func(ctx context.Context, o *opts.ContainerStatsOptions) error {
		o.OneShot = out
		return nil
	})
}

type Output interface {
	// Empty for now, may change in future
}
