package containerd

import (
	"context"
	"io"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/images/archive"
	"github.com/containerd/containerd/platforms"
	"github.com/docker/distribution/reference"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// ExportImage exports a list of images to the given output stream. The
// exported images are archived into a tar when written to the output
// stream. All images with the given tag and all versions containing
// the same tag are exported. names is the set of tags to export, and
// outStream is the writer which the images are written to.
//
// TODO(thaJeztah): produce JSON stream progress response and image events; see https://github.com/moby/moby/issues/43910
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
//
// TODO(thaJeztah): produce JSON stream progress response and image events; see https://github.com/moby/moby/issues/43910
func (i *ImageService) LoadImage(ctx context.Context, inTar io.ReadCloser, outStream io.Writer, quiet bool) error {
	platform := platforms.All
	imgs, err := i.client.Import(ctx, inTar, containerd.WithImportPlatform(platform))

	if err != nil {
		// TODO(thaJeztah): remove this log or change to debug once we can; see https://github.com/moby/moby/pull/43822#discussion_r937502405
		logrus.WithError(err).Warn("failed to import image to containerd")
		return errors.Wrap(err, "failed to import image")
	}

	for _, img := range imgs {
		platformImg := containerd.NewImageWithPlatform(i.client, img, platform)

		unpacked, err := platformImg.IsUnpacked(ctx, i.snapshotter)
		if err != nil {
			// TODO(thaJeztah): remove this log or change to debug once we can; see https://github.com/moby/moby/pull/43822#discussion_r937502405
			logrus.WithError(err).WithField("image", img.Name).Debug("failed to check if image is unpacked")
			continue
		}

		if !unpacked {
			err := platformImg.Unpack(ctx, i.snapshotter)
			if err != nil {
				// TODO(thaJeztah): remove this log or change to debug once we can; see https://github.com/moby/moby/pull/43822#discussion_r937502405
				logrus.WithError(err).WithField("image", img.Name).Warn("failed to unpack image")
				return errors.Wrap(err, "failed to unpack image")
			}
		}
	}
	return nil
}
