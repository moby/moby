package images // import "github.com/docker/docker/daemon/images"

import (
	"context"
	"io"

	"github.com/containerd/log"
	"github.com/docker/docker/container"
	"github.com/docker/docker/image/tarexport"
)

// ExportImage exports a list of images to the given output stream. The
// exported images are archived into a tar when written to the output
// stream. All images with the given tag and all versions containing
// the same tag are exported. names is the set of tags to export, and
// outStream is the writer which the images are written to.
func (i *ImageService) ExportImage(ctx context.Context, names []string, outStream io.Writer) error {
	imageExporter := tarexport.NewTarExporter(i.imageStore, i.layerStore, i.referenceStore, i)
	return imageExporter.Save(ctx, names, outStream)
}

func (i *ImageService) PerformWithBaseFS(ctx context.Context, c *container.Container, fn func(root string) error) error {
	rwlayer, err := i.layerStore.GetRWLayer(c.ID)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			err2 := i.ReleaseLayer(rwlayer)
			if err2 != nil {
				log.G(ctx).WithError(err2).WithField("container", c.ID).Warn("Failed to release layer")
			}
		}
	}()

	basefs, err := rwlayer.Mount(c.GetMountLabel())
	if err != nil {
		return err
	}

	return fn(basefs)
}

// LoadImage uploads a set of images into the repository. This is the
// complement of ExportImage.  The input stream is an uncompressed tar
// ball containing images and metadata.
func (i *ImageService) LoadImage(ctx context.Context, inTar io.ReadCloser, outStream io.Writer, quiet bool) error {
	imageExporter := tarexport.NewTarExporter(i.imageStore, i.layerStore, i.referenceStore, i)
	return imageExporter.Load(ctx, inTar, outStream, quiet)
}
