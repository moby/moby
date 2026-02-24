package exporter

import (
	"context"

	"github.com/moby/buildkit/cache"
	"github.com/moby/buildkit/exporter/containerimage/exptypes"
	"github.com/moby/buildkit/solver/result"
	"github.com/moby/buildkit/util/compression"
	ocispecs "github.com/opencontainers/image-spec/specs-go/v1"
)

type Source = result.Result[cache.ImmutableRef]

type Attestation = result.Attestation[cache.ImmutableRef]

type Exporter interface {
	Resolve(context.Context, int, map[string]string) (ExporterInstance, error)
}

// FinalizeFunc completes an export operation after all exports have created
// their artifacts. It may perform network operations like pushing to a registry.
//
// Calling FinalizeFunc is optional. If not called (e.g., due to cancellation or
// an error in another operation), the export will be incomplete but no resources
// will leak. FinalizeFunc performs completion work only, not cleanup.
//
// FinalizeFunc is safe to call concurrently with other FinalizeFunc calls.
type FinalizeFunc func(ctx context.Context) error

type ExporterInstance interface {
	ID() int
	Name() string
	Config() *Config
	Type() string
	Attrs() map[string]string

	// Export performs the export operation and optionally returns a finalize
	// callback. This separates work that must run sequentially from work that
	// can run in parallel with other exports (e.g., cache export).
	//
	// For exporters that complete all work during Export (tar, local),
	// return nil for the finalize callback.
	Export(ctx context.Context, src *Source, buildInfo ExportBuildInfo) (
		response map[string]string,
		finalize FinalizeFunc,
		ref DescriptorReference,
		err error,
	)
}

type ExportBuildInfo struct {
	Ref         string
	InlineCache exptypes.InlineCache
	SessionID   string
}

type DescriptorReference interface {
	Release() error
	Descriptor() ocispecs.Descriptor
}

type Config struct {
	// Make the field private in case it is initialized with nil compression.Type
	compression compression.Config
}

func NewConfig() *Config {
	return &Config{
		compression: compression.Config{
			Type: compression.Default,
		},
	}
}

func NewConfigWithCompression(comp compression.Config) *Config {
	return &Config{
		compression: comp,
	}
}

func (c *Config) Compression() compression.Config {
	return c.compression
}
