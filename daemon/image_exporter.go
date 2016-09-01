package daemon

import (
	"io"

	"github.com/docker/docker/image/tarexport"
)

// ExportImage exports a list of images to the given output stream. The
// exported images are archived into a tar when written to the output
// stream. All images with the given tag and all versions containing
// the same tag are exported. names is the set of tags to export.
// outStream is the writer which the images are written to. refs is a map used
// when exporting to the OCI format.
// format is the format of the resulting tar ball.
func (daemon *Daemon) ExportImage(names []string, format string, refs map[string]string, outStream io.Writer) error {
	opts := &tarexport.Options{
		Format:       format,
		Refs:         refs,
		Experimental: daemon.HasExperimental(),
	}
	imageExporter := tarexport.NewTarExporter(daemon.imageStore, daemon.layerStore, daemon.referenceStore, daemon, opts)
	return imageExporter.Save(names, outStream)
}

// LoadImage loads a set of images into the repository. This is the
// complement of ImageExport.  The input stream is an uncompressed tar
// ball containing images and metadata.
func (daemon *Daemon) LoadImage(inTar io.ReadCloser, outStream io.Writer, name string, refs map[string]string, quiet bool) error {
	opts := &tarexport.Options{
		Name:         name,
		Refs:         refs,
		Experimental: daemon.HasExperimental(),
	}
	imageExporter := tarexport.NewTarExporter(daemon.imageStore, daemon.layerStore, daemon.referenceStore, daemon, opts)
	// the first arg will be "name" passed down from LoadImage() itself
	return imageExporter.Load(inTar, outStream, quiet)
}
