package images

import (
	"context"
	"io"

	"github.com/moby/moby/v2/daemon/internal/image/tarexport"
	"github.com/moby/moby/v2/errdefs"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

// ExportImage exports a list of images to the given output stream. The
// exported images are archived into a tar when written to the output
// stream. All images with the given tag and all versions containing
// the same tag are exported. names is the set of tags to export, and
// outStream is the writer which the images are written to.
func (i *ImageService) ExportImage(ctx context.Context, names []string, platformList []ocispec.Platform, outStream io.Writer) error {
	var platform *ocispec.Platform

	if len(platformList) > 1 {
		return errdefs.InvalidParameter(errors.New("multiple platforms not supported for this image store; use a multi-platform image store such as containerd-snapshotter"))
	} else if len(platformList) == 1 {
		platform = &platformList[0]
	}

	imageExporter := tarexport.NewTarExporter(i.imageStore, i.layerStore, i.referenceStore, i, platform)
	return imageExporter.Save(ctx, names, outStream)
}

// LoadImage uploads a set of images into the repository. This is the
// complement of ExportImage.  The input stream is an uncompressed tar
// ball containing images and metadata.
func (i *ImageService) LoadImage(ctx context.Context, inTar io.ReadCloser, platformList []ocispec.Platform, outStream io.Writer, quiet bool) error {
	var platform *ocispec.Platform

	if len(platformList) > 1 {
		return errdefs.InvalidParameter(errors.New("multiple platforms not supported for this image store; use a multi-platform image store such as containerd-snapshotter"))
	} else if len(platformList) == 1 {
		platform = &platformList[0]
	}

	imageExporter := tarexport.NewTarExporter(i.imageStore, i.layerStore, i.referenceStore, i, platform)
	return imageExporter.Load(ctx, inTar, outStream, quiet)
}
