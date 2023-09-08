package containerd

import (
	"context"
	"fmt"
	"io"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/content"
	cerrdefs "github.com/containerd/containerd/errdefs"
	containerdimages "github.com/containerd/containerd/images"
	"github.com/containerd/containerd/images/archive"
	"github.com/containerd/containerd/leases"
	"github.com/containerd/containerd/log"
	cplatforms "github.com/containerd/containerd/platforms"
	"github.com/docker/distribution/reference"
	"github.com/docker/docker/container"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/pkg/platforms"
	"github.com/docker/docker/pkg/streamformatter"
	"github.com/opencontainers/image-spec/specs-go"
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
func (i *ImageService) ExportImage(ctx context.Context, names []string, outStream io.Writer) error {
	platform := platforms.AllPlatformsWithPreference(cplatforms.Default())
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
		// Note: This is only applicable if importing this archive into non-containerd Docker.
		// Importing the same archive into containerd, will not restrict the platforms.
		archive.WithPlatform(platform),
	}

	contentStore := i.client.ContentStore()
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

	for _, name := range names {
		target, err := i.resolveDescriptor(ctx, name)
		if err != nil {
			return err
		}

		if err = leaseContent(ctx, contentStore, leasesManager, lease, target); err != nil {
			return err
		}

		// We may not have locally all the platforms that are specified in the index.
		// Export only those manifests that we have.
		// TODO(vvoland): Reconsider this when `--platform` is added.
		if containerdimages.IsIndexType(target.MediaType) {
			desc, err := i.getBestDescriptorForExport(ctx, target)
			if err != nil {
				return err
			}
			target = desc
		}

		if ref, err := reference.ParseNormalizedNamed(name); err == nil {
			ref = reference.TagNameOnly(ref)
			opts = append(opts, archive.WithManifest(target, ref.String()))

			log.G(ctx).WithFields(log.Fields{
				"target": target,
				"name":   ref.String(),
			}).Debug("export image")
		} else {
			opts = append(opts, archive.WithManifest(target))

			log.G(ctx).WithFields(log.Fields{
				"target": target,
			}).Debug("export image without name")
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
func (i *ImageService) LoadImage(ctx context.Context, inTar io.ReadCloser, outStream io.Writer, quiet bool) error {
	opts := []containerd.ImportOpt{
		// TODO(vvoland): Allow user to pass platform
		containerd.WithImportPlatform(cplatforms.All),

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

	imgs, err := i.client.Import(ctx, inTar, opts...)
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
			name = reference.FamiliarName(reference.TagNameOnly(named))
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
		if err != nil {
			return errors.Wrap(err, "failed to unpack loaded image")
		}

		fmt.Fprintf(progress, "%s: %s\n", loadedMsg, name)
		i.LogImageEvent(img.Target.Digest.String(), img.Target.Digest.String(), "load")
	}

	return nil
}

// getBestDescriptorForExport returns a descriptor which only references content available locally.
// The returned descriptor can be:
// - The same index descriptor - if all content is available
// - Platform specific manifest - if only one manifest from the whole index is available
// - Reduced index descriptor - if not all, but more than one manifest is available
//
// The reduced index descriptor is stored in the content store and may be garbage collected.
// It's advised to pass a context with a lease that's long enough to cover usage of the blob.
func (i *ImageService) getBestDescriptorForExport(ctx context.Context, indexDesc ocispec.Descriptor) (ocispec.Descriptor, error) {
	none := ocispec.Descriptor{}

	if !containerdimages.IsIndexType(indexDesc.MediaType) {
		err := fmt.Errorf("index/manifest-list descriptor expected, got: %s", indexDesc.MediaType)
		return none, errdefs.InvalidParameter(err)
	}
	store := i.client.ContentStore()
	children, err := containerdimages.Children(ctx, store, indexDesc)
	if err != nil {
		if cerrdefs.IsNotFound(err) {
			return none, errdefs.NotFound(err)
		}
		return none, errdefs.System(err)
	}

	// Check which platform manifests have all their blobs available.
	hasMissingManifests := false
	var presentManifests []ocispec.Descriptor
	for _, mfst := range children {
		if containerdimages.IsManifestType(mfst.MediaType) {
			available, _, _, missing, err := containerdimages.Check(ctx, store, mfst, nil)
			if err != nil {
				hasMissingManifests = true
				log.G(ctx).WithField("manifest", mfst.Digest).Warn("failed to check manifest's blob availability, won't export")
				continue
			}

			if available && len(missing) == 0 {
				presentManifests = append(presentManifests, mfst)
				log.G(ctx).WithField("manifest", mfst.Digest).Debug("manifest content present, will export")
			} else {
				hasMissingManifests = true
				log.G(ctx).WithFields(log.Fields{
					"manifest": mfst.Digest,
					"missing":  missing,
				}).Debug("manifest is missing, won't export")
			}
		}
	}

	if !hasMissingManifests || len(children) == 0 {
		// If we have the full image, or it has no manifests, just export the original index.
		return indexDesc, nil
	} else if len(presentManifests) == 1 {
		// If only one platform is present, export that one manifest.
		return presentManifests[0], nil
	} else if len(presentManifests) == 0 {
		// Return error when none of the image's manifest is present.
		return none, errdefs.NotFound(fmt.Errorf("none of the manifests is fully present in the content store"))
	}

	// Create a new index which contains only the manifests we have in store.
	index := ocispec.Index{
		Versioned: specs.Versioned{
			SchemaVersion: 2,
		},
		MediaType:   ocispec.MediaTypeImageIndex,
		Manifests:   presentManifests,
		Annotations: indexDesc.Annotations,
	}

	reducedIndexDesc, err := storeJson(ctx, store, index.MediaType, index, nil)
	if err != nil {
		return none, err
	}

	return reducedIndexDesc, nil
}
