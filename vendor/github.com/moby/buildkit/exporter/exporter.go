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
	Export(context.Context, Source) (map[string]string, error)
}

type Source struct {
	Ref      cache.ImmutableRef
	Refs     map[string]cache.ImmutableRef
	Metadata map[string][]byte
}
