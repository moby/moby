package exporter

import (
	"context"

	"github.com/moby/buildkit/cache"
)

type Exporter interface {
	Resolve(context.Context, map[string]string) (ExporterInstance, error)
}

type ExporterInstance interface {
	Name() string
	Export(context.Context, cache.ImmutableRef, map[string][]byte) (map[string]string, error)
}
