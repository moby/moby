package streams

import (
	"context"

	"github.com/docker/docker/api/types/streams"
)

type Backend interface {
	Create(ctx context.Context, id string, spec streams.Spec) (*streams.Stream, error)
	Get(ctx context.Context, id string) (*streams.Stream, error)
	Delete(ctx context.Context, id string) error
}
