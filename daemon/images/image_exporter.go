package images // import "github.com/docker/docker/daemon/images"

import (
	"context"
	"io"

	"github.com/docker/docker/image/tarexport"
)

// SaveImage exports a list of images to the given output stream. The
// exported images are archived into a tar when written to the output
// stream. All images with the given tag and all versions containing
// the same tag are exported. names is the set of tags to export, and
// outStream is the writer which the images are written to.
func (i *ImageService) SaveImage(ctx context.Context, names []string, outStream io.Writer) error {
	imageExporter := tarexport.NewTarExporter(i.imageStore, i.layerStore, i.referenceStore, i)
	return imageExporter.Save(names, outStream)
}

// LoadImage uploads a set of images into the repository. This is the
// complement of SaveImage.  The input stream is an uncompressed tar
// ball containing images and metadata.
func (i *ImageService) LoadImage(ctx context.Context, inTar io.ReadCloser, outStream io.Writer, quiet bool) error {
	imageExporter := tarexport.NewTarExporter(i.imageStore, i.layerStore, i.referenceStore, i)
	return imageExporter.Load(inTar, outStream, quiet)
}
