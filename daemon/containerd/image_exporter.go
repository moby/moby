package containerd

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/content"
	containerdimages "github.com/containerd/containerd/images"
	"github.com/containerd/containerd/images/archive"
	"github.com/containerd/containerd/leases"
	cerrdefs "github.com/containerd/errdefs"
	"github.com/containerd/log"
	"github.com/containerd/platforms"
	"github.com/distribution/reference"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/container"
	"github.com/docker/docker/daemon/images"
	"github.com/docker/docker/errdefs"
	dockerarchive "github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/streamformatter"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

func (i *ImageService) PerformWithBaseFS(ctx context.Context, c *container.Container, fn func(root string) error) error {
	snapshotter := i.client.SnapshotService(c.Driver)
	mounts, err := snapshotter.Mounts(ctx, c.ID)
	if err != nil {
		return err
	}
	path, err := i.refCountMounter.Mount(mounts, c.ID)
	if err != nil {
		return err
	}
	defer i.refCountMounter.Unmount(path)

	return fn(path)
}

// ExportImage exports a list of images to the given output stream. The
// exported images are archived into a tar when written to the output
// stream. All images with the given tag and all versions containing
// the same tag are exported. names is the set of tags to export, and
// outStream is the writer which the images are written to.
//
// TODO(thaJeztah): produce JSON stream progress response and image events; see https://github.com/moby/moby/issues/43910
func (i *ImageService) ExportImage(ctx context.Context, names []string, platform *ocispec.Platform, outStream io.Writer) error {
	pm := i.matchRequestedOrDefault(platforms.OnlyStrict, platform)

	opts := []archive.ExportOpt{
		archive.WithSkipNonDistributableBlobs(),

		// This makes the exported archive also include `manifest.json`
		// when the image is a manifest list. It is needed for backwards
		// compatibility with Docker image format.
		// The containerd will choose only one manifest for the `manifest.json`.
		// Our preference is to have it point to the default platform.
		// Example:
		//  Daemon is running on linux/arm64
		//  When we export linux/amd64 and linux/arm64, manifest.json will point to linux/arm64.
		//  When we export linux/amd64 only, manifest.json will point to linux/amd64.
		// Note: This only matters when importing this archive into non-containerd Docker.
		// Importing the same archive into containerd, will not restrict the platforms.
		archive.WithPlatform(pm),
		archive.WithSkipMissing(i.content),
	}

	leasesManager := i.client.LeasesService()
	lease, err := leasesManager.Create(ctx, leases.WithRandomID())
	if err != nil {
		return errdefs.System(err)
	}
	defer func() {
		if err := leasesManager.Delete(ctx, lease); err != nil {
			log.G(ctx).WithError(err).Warn("cleaning up lease")
		}
	}()

	addLease := func(ctx context.Context, target ocispec.Descriptor) error {
		return leaseContent(ctx, i.content, leasesManager, lease, target)
	}

	exportImage := func(ctx context.Context, img containerdimages.Image, ref reference.Named) error {
		target := img.Target

		if platform != nil {
			newTarget, err := i.getPushDescriptor(ctx, img, platform)
			if err != nil {
				return errors.Wrap(err, "no suitable export target found for platform "+platforms.FormatAll(*platform))
			}
			target = newTarget
		}

		if err := addLease(ctx, target); err != nil {
			return err
		}

		if ref != nil {
			opts = append(opts, archive.WithManifest(target, ref.String()))

			log.G(ctx).WithFields(log.Fields{
				"target": target,
				"name":   ref,
			}).Debug("export image")
		} else {
			orgTarget := target
			target.Annotations = make(map[string]string)

			for k, v := range orgTarget.Annotations {
				switch k {
				case containerdimages.AnnotationImageName, ocispec.AnnotationRefName:
					// Strip image name/tag annotations from the descriptor.
					// Otherwise containerd will use it as name.
				default:
					target.Annotations[k] = v
				}
			}

			opts = append(opts, archive.WithManifest(target))

			log.G(ctx).WithFields(log.Fields{
				"target": target,
			}).Debug("export image without name")
		}

		i.LogImageEvent(target.Digest.String(), target.Digest.String(), events.ActionSave)
		return nil
	}

	exportRepository := func(ctx context.Context, ref reference.Named) error {
		imgs, err := i.getAllImagesWithRepository(ctx, ref)
		if err != nil {
			return errdefs.System(fmt.Errorf("failed to list all images from repository %s: %w", ref.Name(), err))
		}

		if len(imgs) == 0 {
			return images.ErrImageDoesNotExist{Ref: ref}
		}

		for _, img := range imgs {
			ref, err := reference.ParseNamed(img.Name)
			if err != nil {
				log.G(ctx).WithFields(log.Fields{
					"image": img.Name,
					"error": err,
				}).Warn("couldn't parse image name as a valid named reference")
				continue
			}

			if err := exportImage(ctx, img, ref); err != nil {
				return err
			}
		}

		return nil
	}

	for _, name := range names {
		img, resolveErr := i.resolveImage(ctx, name)

		// Check if the requested name is a truncated digest of the resolved descriptor.
		// If yes, that means that the user specified a specific image ID so
		// it's not referencing a repository.
		specificDigestResolved := false
		if resolveErr == nil {
			nameWithoutDigestAlgorithm := strings.TrimPrefix(name, img.Target.Digest.Algorithm().String()+":")
			specificDigestResolved = strings.HasPrefix(img.Target.Digest.Encoded(), nameWithoutDigestAlgorithm)
		}

		log.G(ctx).WithFields(log.Fields{
			"name":                   name,
			"img":                    img,
			"resolveErr":             resolveErr,
			"specificDigestResolved": specificDigestResolved,
		}).Debug("export requested")

		ref, refErr := reference.ParseNormalizedNamed(name)

		if refErr == nil {
			if _, ok := ref.(reference.Digested); ok {
				specificDigestResolved = true
			}
		}

		if resolveErr != nil || !specificDigestResolved {
			// Name didn't resolve to anything, or name wasn't explicitly referencing a digest
			if refErr == nil && reference.IsNameOnly(ref) {
				// Reference is valid, but doesn't include a specific tag.
				// Export all images with the same repository.
				if err := exportRepository(ctx, ref); err != nil {
					return err
				}
				continue
			}
		}

		if resolveErr != nil {
			return resolveErr
		}
		if refErr != nil {
			return refErr
		}

		// If user exports a specific digest, it shouldn't have a tag.
		if specificDigestResolved {
			ref = nil
		}

		if err := exportImage(ctx, img, ref); err != nil {
			return err
		}
	}

	return i.client.Export(ctx, outStream, opts...)
}

// leaseContent will add a resource to the lease for each child of the descriptor making sure that it and
// its children won't be deleted while the lease exists
func leaseContent(ctx context.Context, store content.Store, leasesManager leases.Manager, lease leases.Lease, desc ocispec.Descriptor) error {
	return containerdimages.Walk(ctx, containerdimages.HandlerFunc(func(ctx context.Context, desc ocispec.Descriptor) ([]ocispec.Descriptor, error) {
		_, err := store.Info(ctx, desc.Digest)
		if err != nil {
			if errors.Is(err, cerrdefs.ErrNotFound) {
				return nil, nil
			}
			return nil, errdefs.System(err)
		}

		r := leases.Resource{
			ID:   desc.Digest.String(),
			Type: "content",
		}
		if err := leasesManager.AddResource(ctx, lease, r); err != nil {
			return nil, errdefs.System(err)
		}

		return containerdimages.Children(ctx, store, desc)
	}), desc)
}

// LoadImage uploads a set of images into the repository. This is the
// complement of ExportImage.  The input stream is an uncompressed tar
// ball containing images and metadata.
func (i *ImageService) LoadImage(ctx context.Context, inTar io.ReadCloser, platform *ocispec.Platform, outStream io.Writer, quiet bool) error {
	decompressed, err := dockerarchive.DecompressStream(inTar)
	if err != nil {
		return errors.Wrap(err, "failed to decompress input tar archive")
	}
	defer decompressed.Close()

	pm := i.matchRequestedOrDefault(platforms.OnlyStrict, platform)

	opts := []containerd.ImportOpt{
		containerd.WithImportPlatform(pm),

		containerd.WithSkipMissing(),

		// Create an additional image with dangling name for imported images...
		containerd.WithDigestRef(danglingImageName),
		// ... but only if they don't have a name or it's invalid.
		containerd.WithSkipDigestRef(func(nameFromArchive string) bool {
			if nameFromArchive == "" {
				return false
			}
			_, err := reference.ParseNormalizedNamed(nameFromArchive)
			return err == nil
		}),
	}

	imgs, err := i.client.Import(ctx, decompressed, opts...)
	if err != nil {
		log.G(ctx).WithError(err).Debug("failed to import image to containerd")
		return errdefs.System(err)
	}

	progress := streamformatter.NewStdoutWriter(outStream)

	for _, img := range imgs {
		name := img.Name
		loadedMsg := "Loaded image"

		if isDanglingImage(img) {
			name = img.Target.Digest.String()
			loadedMsg = "Loaded image ID"
		} else if named, err := reference.ParseNormalizedNamed(img.Name); err == nil {
			name = reference.FamiliarString(reference.TagNameOnly(named))
		}

		err = i.walkImageManifests(ctx, img, func(platformImg *ImageManifest) error {
			logger := log.G(ctx).WithFields(log.Fields{
				"image":    name,
				"manifest": platformImg.Target().Digest,
			})

			if isPseudo, err := platformImg.IsPseudoImage(ctx); isPseudo || err != nil {
				if err != nil {
					logger.WithError(err).Warn("failed to read manifest")
				} else {
					logger.Debug("don't unpack non-image manifest")
				}
				return nil
			}

			imgPlat, err := platformImg.ImagePlatform(ctx)
			if err != nil {
				logger.WithError(err).Warn("failed to read image platform, skipping unpack")
				return nil
			}

			// Only unpack the image if it matches the host platform
			if !i.hostPlatformMatcher().Match(imgPlat) {
				return nil
			}

			unpacked, err := platformImg.IsUnpacked(ctx, i.snapshotter)
			if err != nil {
				logger.WithError(err).Warn("failed to check if image is unpacked")
				return nil
			}

			if !unpacked {
				err = platformImg.Unpack(ctx, i.snapshotter)
				if err != nil {
					return errdefs.System(err)
				}
			}
			logger.WithField("alreadyUnpacked", unpacked).WithError(err).Debug("unpack")
			return nil
		})

		fmt.Fprintf(progress, "%s: %s\n", loadedMsg, name)
		i.LogImageEvent(img.Target.Digest.String(), img.Target.Digest.String(), events.ActionLoad)

		if err != nil {
			// The image failed to unpack, but is already imported, log the error but don't fail the whole load.
			fmt.Fprintf(progress, "Error unpacking image %s: %v\n", name, err)
		}
	}

	return nil
}
