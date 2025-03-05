package images // import "github.com/docker/docker/daemon/images"

import (
	"context"
	"io"

	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/image/tarexport"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

// ExportImage exports a list of images to the given output stream. The
// exported images are archived into a tar when written to the output
// stream. All images with the given tag and all versions containing
// the same tag are exported. names is the set of tags to export, and
// outStream is the writer which the images are written to.
func (i *ImageService) ExportImage(ctx context.Context, names []string, platformSpecs []ocispec.Platform, outStream io.Writer) error {
	if len(platformSpecs) > 1 {
		return errdefs.InvalidParameter(errors.New("multiple platform parameters not supported"))
	}
	var platform *ocispec.Platform
	if len(platformSpecs) == 1 {
		platform = &platformSpecs[0]
	}
	imageExporter := tarexport.NewTarExporter(i.imageStore, i.layerStore, i.referenceStore, i, platform)
	return imageExporter.Save(ctx, names, outStream)
}

// LoadImage uploads a set of images into the repository. This is the
// complement of ExportImage.  The input stream is an uncompressed tar
// ball containing images and metadata.
func (i *ImageService) LoadImage(ctx context.Context, inTar io.ReadCloser, platformSpecs []ocispec.Platform, outStream io.Writer, quiet bool) error {
	if len(platformSpecs) > 1 {
		return errdefs.InvalidParameter(errors.New("multiple platform parameters not supported"))
	}
	var platform *ocispec.Platform
	if len(platformSpecs) == 1 {
		platform = &platformSpecs[0]
	}
	imageExporter := tarexport.NewTarExporter(i.imageStore, i.layerStore, i.referenceStore, i, platform)
	return imageExporter.Load(ctx, inTar, outStream, quiet)
}
