package containerd

import (
	"context"
	"io"

	"github.com/containerd/containerd"
	cerrdefs "github.com/containerd/containerd/errdefs"
	containerdimages "github.com/containerd/containerd/images"
	"github.com/containerd/containerd/images/archive"
	"github.com/containerd/containerd/images/converter"
	"github.com/containerd/containerd/platforms"
	"github.com/docker/distribution/reference"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
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
		archive.WithSkipNonDistributableBlobs(),
	}

	for _, imageRef := range names {
		var err error
		opts, err = i.appendImageForExport(ctx, opts, imageRef)
		if err != nil {
			return err
		}
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

		unpacked, err := platformImg.IsUnpacked(ctx, containerd.DefaultSnapshotter)
		if err != nil {
			// TODO(thaJeztah): remove this log or change to debug once we can; see https://github.com/moby/moby/pull/43822#discussion_r937502405
			logrus.WithError(err).WithField("image", img.Name).Debug("failed to check if image is unpacked")
			continue
		}

		if !unpacked {
			err := platformImg.Unpack(ctx, containerd.DefaultSnapshotter)
			if err != nil {
				// TODO(thaJeztah): remove this log or change to debug once we can; see https://github.com/moby/moby/pull/43822#discussion_r937502405
				logrus.WithError(err).WithField("image", img.Name).Warn("failed to unpack image")
				return errors.Wrap(err, "failed to unpack image")
			}
		}
	}
	return nil
}

func (i *ImageService) appendImageForExport(ctx context.Context, opts []archive.ExportOpt, name string) ([]archive.ExportOpt, error) {
	ref, err := reference.ParseDockerRef(name)
	if err != nil {
		return opts, err
	}

	is := i.client.ImageService()

	img, err := is.Get(ctx, ref.String())
	if err != nil {
		return opts, err
	}

	store := i.client.ContentStore()

	if containerdimages.IsIndexType(img.Target.MediaType) {
		children, err := containerdimages.Children(ctx, store, img.Target)
		if err != nil {
			return opts, err
		}

		// Check which platform manifests we have blobs for.
		missingPlatforms := []v1.Platform{}
		presentPlatforms := []v1.Platform{}
		for _, child := range children {
			if containerdimages.IsManifestType(child.MediaType) {
				_, err := store.ReaderAt(ctx, child)
				if cerrdefs.IsNotFound(err) {
					missingPlatforms = append(missingPlatforms, *child.Platform)
					logrus.WithField("digest", child.Digest.String()).Debug("missing blob, not exporting")
					continue
				} else if err != nil {
					return opts, err
				}
				presentPlatforms = append(presentPlatforms, *child.Platform)
			}
		}

		// If we have all the manifests, just export the original index.
		if len(missingPlatforms) == 0 {
			return append(opts, archive.WithImage(is, img.Name)), nil
		}

		// Create a new manifest which contains only the manifests we have in store.
		srcRef := ref.String()
		targetRef := srcRef + "-tmp-export"
		newImg, err := converter.Convert(ctx, i.client, targetRef, srcRef,
			converter.WithPlatform(platforms.Any(presentPlatforms...)))
		if err != nil {
			return opts, err
		}
		defer i.client.ImageService().Delete(ctx, newImg.Name, containerdimages.SynchronousDelete())
		return append(opts, archive.WithManifest(newImg.Target, srcRef)), nil
	}

	return append(opts, archive.WithImage(is, img.Name)), nil
}
