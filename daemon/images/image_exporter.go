package images // import "github.com/docker/docker/daemon/images"

import (
	"io"

	"github.com/containerd/containerd/errdefs"
)

// ExportImage exports a list of images to the given output stream. The
// exported images are archived into a tar when written to the output
// stream. All images with the given tag and all versions containing
// the same tag are exported. names is the set of tags to export, and
// outStream is the writer which the images are written to.
func (i *ImageService) ExportImage(names []string, outStream io.Writer) error {
	// TODO(containerd): use containerd's archive exporter?
	// This may require special logic to output the Docker format
	//imageExporter := tarexport.NewTarExporter(i.imageStore, i.layerStores, i.referenceStore, i)
	//return imageExporter.Save(names, outStream)
	return errdefs.ErrNotImplemented
}
