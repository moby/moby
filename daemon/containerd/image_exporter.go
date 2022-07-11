package containerd

import (
	"context"
	"io"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/images/archive"
	"github.com/containerd/containerd/platforms"
	"github.com/docker/distribution/reference"
)

// ExportImage exports a list of images to the given output stream. The
// exported images are archived into a tar when written to the output
// stream. All images with the given tag and all versions containing
// the same tag are exported. names is the set of tags to export, and
// outStream is the writer which the images are written to.
func (i *ImageService) ExportImage(ctx context.Context, names []string, outStream io.Writer) error {
	opts := []archive.ExportOpt{
		archive.WithPlatform(platforms.Ordered(platforms.DefaultSpec())),
		archive.WithSkipNonDistributableBlobs(),
	}
	is := i.client.ImageService()
	for _, imageRef := range names {
		named, err := reference.ParseDockerRef(imageRef)
		if err != nil {
			return err
		}
		opts = append(opts, archive.WithImage(is, named.String()))
	}
	return i.client.Export(ctx, outStream, opts...)
}

// LoadImage uploads a set of images into the repository. This is the
// complement of ExportImage.  The input stream is an uncompressed tar
// ball containing images and metadata.
func (i *ImageService) LoadImage(ctx context.Context, inTar io.ReadCloser, outStream io.Writer, quiet bool) error {
	_, err := i.client.Import(ctx, inTar,
		containerd.WithImportPlatform(platforms.DefaultStrict()),
	)
	return err
}
