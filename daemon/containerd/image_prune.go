package containerd

import (
	"context"
	"strings"

	containerdimages "github.com/containerd/containerd/images"
	cerrdefs "github.com/containerd/errdefs"
	"github.com/containerd/log"
	"github.com/distribution/reference"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/errdefs"
	"github.com/hashicorp/go-multierror"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

var imagesAcceptedFilters = map[string]bool{
	"dangling": true,
	"label":    true,
	"label!":   true,
	"until":    true,
}

// errPruneRunning is returned when a prune request is received while
// one is in progress
var errPruneRunning = errdefs.Conflict(errors.New("a prune operation is already running"))

// ImagesPrune removes unused images
func (i *ImageService) ImagesPrune(ctx context.Context, fltrs filters.Args) (*image.PruneReport, error) {
	if !i.pruneRunning.CompareAndSwap(false, true) {
		return nil, errPruneRunning
	}
	defer i.pruneRunning.Store(false)

	err := fltrs.Validate(imagesAcceptedFilters)
	if err != nil {
		return nil, err
	}

	danglingOnly, err := fltrs.GetBoolOrDefault("dangling", true)
	if err != nil {
		return nil, err
	}
	// dangling=false will filter out dangling images like in image list.
	// Remove it, because in this context dangling=false means that we're
	// pruning NOT ONLY dangling (`docker image prune -a`) instead of NOT DANGLING.
	// This will be handled by the danglingOnly parameter of pruneUnused.
	for _, v := range fltrs.Get("dangling") {
		fltrs.Del("dangling", v)
	}

	filterFunc, err := i.setupFilters(ctx, fltrs)
	if err != nil {
		return nil, err
	}

	return i.pruneUnused(ctx, filterFunc, danglingOnly)
}

func (i *ImageService) pruneUnused(ctx context.Context, filterFunc imageFilterFunc, danglingOnly bool) (*image.PruneReport, error) {
	report := image.PruneReport{}

	allImages, err := i.images.List(ctx)
	if err != nil {
		return nil, err
	}

	// How many images make reference to a particular target digest.
	digestRefCount := map[digest.Digest]int{}
	// Images considered for pruning.
	imagesToPrune := map[string]containerdimages.Image{}
	for _, img := range allImages {
		digestRefCount[img.Target.Digest] += 1

		if !danglingOnly || isDanglingImage(img) {
			canBePruned := filterFunc(img)
			log.G(ctx).WithFields(log.Fields{
				"image":       img.Name,
				"canBePruned": canBePruned,
			}).Debug("considering image for pruning")

			if canBePruned {
				imagesToPrune[img.Name] = img
			}

		}
	}

	// Image specified by digests that are used by containers.
	usedDigests := map[digest.Digest]struct{}{}

	// Exclude images used by existing containers
	for _, ctr := range i.containers.List() {
		// If the original image was deleted, make sure we don't delete the dangling image
		delete(imagesToPrune, danglingImageName(ctr.ImageID.Digest()))

		// Config.Image is the image reference passed by user.
		// Config.ImageID is the resolved content digest based on the user's Config.Image.
		// For example: container created by:
		//           `docker run alpine` will have Config.Image="alpine"
		//           `docker run 82d1e9d` will have Config.Image="82d1e9d"
		// but both will have ImageID="sha256:82d1e9d7ed48a7523bdebc18cf6290bdb97b82302a8a9c27d4fe885949ea94d1"
		imageDgst := ctr.ImageID.Digest()

		// If user didn't specify an explicit image, mark the digest as used.
		normalizedImageID := "sha256:" + strings.TrimPrefix(ctr.Config.Image, "sha256:")
		if strings.HasPrefix(imageDgst.String(), normalizedImageID) {
			usedDigests[imageDgst] = struct{}{}
			continue
		}

		ref, err := reference.ParseNormalizedNamed(ctr.Config.Image)
		log.G(ctx).WithFields(log.Fields{
			"ctr":          ctr.ID,
			"image":        ref,
			"nameParseErr": err,
		}).Debug("filtering container's image")

		if err == nil {
			// If user provided a specific image name, exclude that image.
			name := reference.TagNameOnly(ref)
			delete(imagesToPrune, name.String())
		}
	}

	// Create dangling images for images that will be deleted but are still in use.
	for _, img := range imagesToPrune {
		dgst := img.Target.Digest

		digestRefCount[dgst] -= 1
		if digestRefCount[dgst] == 0 {
			if _, isUsed := usedDigests[dgst]; isUsed {
				if err := i.ensureDanglingImage(ctx, img); err != nil {
					return &report, errors.Wrapf(err, "failed to create ensure dangling image for %s", img.Name)
				}
			}
		}
	}

	possiblyDeletedConfigs := map[digest.Digest]struct{}{}
	var errs error

	// Workaround for https://github.com/moby/buildkit/issues/3797
	defer func() {
		if err := i.unleaseSnapshotsFromDeletedConfigs(context.WithoutCancel(ctx), possiblyDeletedConfigs); err != nil {
			errs = multierror.Append(errs, err)
		}
	}()

	for _, img := range imagesToPrune {
		log.G(ctx).WithField("image", img).Debug("pruning image")

		blobs := []ocispec.Descriptor{}

		err := i.walkPresentChildren(ctx, img.Target, func(_ context.Context, desc ocispec.Descriptor) error {
			blobs = append(blobs, desc)
			if containerdimages.IsConfigType(desc.MediaType) {
				possiblyDeletedConfigs[desc.Digest] = struct{}{}
			}
			return nil
		})
		if err != nil {
			errs = multierror.Append(errs, err)
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return &report, errs
			}
			continue
		}
		err = i.images.Delete(ctx, img.Name, containerdimages.SynchronousDelete())
		if err != nil && !cerrdefs.IsNotFound(err) {
			errs = multierror.Append(errs, err)
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return &report, errs
			}
			continue
		}

		report.ImagesDeleted = append(report.ImagesDeleted,
			image.DeleteResponse{
				Untagged: imageFamiliarName(img),
			},
		)

		// Check which blobs have been deleted and sum their sizes
		for _, blob := range blobs {
			_, err := i.content.ReaderAt(ctx, blob)

			if cerrdefs.IsNotFound(err) {
				report.ImagesDeleted = append(report.ImagesDeleted,
					image.DeleteResponse{
						Deleted: blob.Digest.String(),
					},
				)
				report.SpaceReclaimed += uint64(blob.Size)
			}
		}
	}

	return &report, errs
}

// unleaseSnapshotsFromDeletedConfigs removes gc.ref.snapshot content label from configs that are not
// referenced by any of the existing images.
// This is a temporary solution to the rootfs snapshot not being deleted when there's a buildkit history
// item referencing an image config.
func (i *ImageService) unleaseSnapshotsFromDeletedConfigs(ctx context.Context, possiblyDeletedConfigs map[digest.Digest]struct{}) error {
	all, err := i.images.List(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to list images during snapshot lease removal")
	}

	var errs error
	for _, img := range all {
		err := i.walkPresentChildren(ctx, img.Target, func(_ context.Context, desc ocispec.Descriptor) error {
			if containerdimages.IsConfigType(desc.MediaType) {
				delete(possiblyDeletedConfigs, desc.Digest)
			}
			return nil
		})
		if err != nil {
			errs = multierror.Append(errs, err)
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return errs
			}
			continue
		}
	}

	// At this point, all configs that are used by any image has been removed from the slice
	for cfgDigest := range possiblyDeletedConfigs {
		info, err := i.content.Info(ctx, cfgDigest)
		if err != nil {
			if cerrdefs.IsNotFound(err) {
				log.G(ctx).WithField("config", cfgDigest).Debug("config already gone")
			} else {
				errs = multierror.Append(errs, err)
				if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
					return errs
				}
			}
			continue
		}

		label := "containerd.io/gc.ref.snapshot." + i.StorageDriver()

		delete(info.Labels, label)
		_, err = i.content.Update(ctx, info, "labels."+label)
		if err != nil {
			errs = multierror.Append(errs, errors.Wrapf(err, "failed to remove gc.ref.snapshot label from %s", cfgDigest))
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return errs
			}
		}
	}

	return errs
}
