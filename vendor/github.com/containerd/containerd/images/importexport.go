package images

import (
	"context"
	"io"

	"github.com/containerd/containerd/content"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// Importer is the interface for image importer.
type Importer interface {
	// Import imports an image from a tar stream.
	Import(ctx context.Context, store content.Store, reader io.Reader) ([]Image, error)
}

// Exporter is the interface for image exporter.
type Exporter interface {
	// Export exports an image to a tar stream.
	Export(ctx context.Context, store content.Store, desc ocispec.Descriptor, writer io.Writer) error
}
