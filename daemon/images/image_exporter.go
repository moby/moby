package images // import "github.com/docker/docker/daemon/images"

import (
	"context"
	"io"

	"github.com/docker/docker/image/tarexport"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// ExportImage exports a list of images to the given output stream. The
// exported images are archived into a tar when written to the output
// stream. All images with the given tag and all versions containing
// the same tag are exported. names is the set of tags to export, and
// outStream is the writer which the images are written to.
func (i *ImageService) ExportImage(ctx context.Context, names []string, platform *ocispec.Platform, outStream io.Writer) error {
	imageExporter := tarexport.NewTarExporter(i.imageStore, i.layerStore, i.referenceStore, i, platform)
	return imageExporter.Save(ctx, names, outStream)
}

// LoadImage uploads a set of images into the repository. This is the
// complement of ExportImage.  The input stream is an uncompressed tar
// ball containing images and metadata.
func (i *ImageService) LoadImage(ctx context.Context, inTar io.ReadCloser, platform *ocispec.Platform, outStream io.Writer, quiet bool) error {
	imageExporter := tarexport.NewTarExporter(i.imageStore, i.layerStore, i.referenceStore, i, platform)
	return imageExporter.Load(ctx, inTar, outStream, quiet)
}
