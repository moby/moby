package containerd

import (
	"context"
	"sort"
	"strings"

	containerdimages "github.com/containerd/containerd/images"
	"github.com/containerd/containerd/leases"
	"github.com/containerd/containerd/tracing"
	cerrdefs "github.com/containerd/errdefs"
	"github.com/containerd/log"
	"github.com/distribution/reference"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/container"
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

	// Prune leases
	leaseManager := i.client.LeasesService()
	pullLeases, err := leaseManager.List(ctx, pruneLeaseFilter)
	if err != nil {
		return nil, err
	}
	for i, lease := range pullLeases {
		var opts []leases.DeleteOpt
		if i == len(pullLeases)-1 {
			opts = append(opts, leases.SynchronousDelete)
		}
		if err := leaseManager.Delete(ctx, lease, opts...); err != nil {
			return nil, err
		}
	}

	return i.pruneUnused(ctx, filterFunc, danglingOnly)
}

// pruneUnused deletes images that are dangling or unused by any container.
// The behavior is controlled by the danglingOnly parameter.
// If danglingOnly is true, only dangling images are deleted.
// Otherwise, all images unused by any container are deleted.
//
// Additionally, the filterFunc parameter is used to filter images that should
// be considered for deletion.
//
// Container created with images specified by an ID only (e.g. `docker run 82d1e9d`)
// will keep at least one image tag with that ID.
//
// In case a digested and tagged reference was used (e.g. `docker run alpine:latest@sha256:82d1e9d7ed48a7523bdebc18cf6290bdb97b82302a8a9c27d4fe885949ea94d1`),
// the alpine:latest image will be kept.
func (i *ImageService) pruneUnused(ctx context.Context, filterFunc imageFilterFunc, danglingOnly bool) (*image.PruneReport, error) {
	ctx, span := tracing.StartSpan(ctx, "ImageService.pruneUnused")
	span.SetAttributes(tracing.Attribute("danglingOnly", danglingOnly))
	defer span.End()

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

	usedDigests := filterImagesUsedByContainers(ctx, i.containers.List(), imagesToPrune)

	// Sort images by name to make the behavior deterministic and consistent with graphdrivers.
	sorted := make([]string, 0, len(imagesToPrune))
	for name := range imagesToPrune {
		sorted = append(sorted, name)
	}
	sort.Strings(sorted)

	// Make sure we don't delete the last image of a particular digest used by any container.
	for _, name := range sorted {
		img := imagesToPrune[name]
		dgst := img.Target.Digest

		if digestRefCount[dgst] > 1 {
			digestRefCount[dgst] -= 1
			continue
		}

		if _, isUsed := usedDigests[dgst]; isUsed {
			delete(imagesToPrune, name)
		}
	}

	return i.pruneAll(ctx, imagesToPrune)
}

// filterImagesUsedByContainers removes image names that are used by containers
// and returns a map of used image digests.
func filterImagesUsedByContainers(ctx context.Context,
	allContainers []*container.Container,
	imagesToPrune map[string]containerdimages.Image,
) (usedDigests map[digest.Digest]struct{}) {
	ctx, span := tracing.StartSpan(ctx, "filterImagesUsedByContainers")
	span.SetAttributes(tracing.Attribute("count", len(allContainers)))
	defer span.End()

	// Image specified by digests that are used by containers.
	usedDigests = map[digest.Digest]struct{}{}

	// Exclude images used by existing containers
	for _, ctr := range allContainers {
		// If the original image was force deleted, make sure we don't delete the dangling image
		delete(imagesToPrune, danglingImageName(ctr.ImageID.Digest()))

		// Config.Image is the image reference passed by user.
		// Config.ImageID is the resolved content digest based on the user's Config.Image.
		// For example: container created by:
		//           `docker run alpine` will have Config.Image="alpine"
		//           `docker run 82d1e9d` will have Config.Image="82d1e9d"
		// but both will have ImageID="sha256:82d1e9d7ed48a7523bdebc18cf6290bdb97b82302a8a9c27d4fe885949ea94d1"
		imageDgst := ctr.ImageID.Digest()

		// If user used an full or truncated ID instead of an explicit image name, mark the digest as used.
		normalizedImageID := "sha256:" + strings.TrimPrefix(ctr.Config.Image, "sha256:")
		fullOrTruncatedID := strings.HasPrefix(imageDgst.String(), normalizedImageID)
		digestedRef := strings.HasSuffix(ctr.Config.Image, "@"+imageDgst.String())
		if fullOrTruncatedID || digestedRef {
			usedDigests[imageDgst] = struct{}{}
		}

		ref, err := reference.ParseNormalizedNamed(ctr.Config.Image)
		log.G(ctx).WithFields(log.Fields{
			"ctr":          ctr.ID,
			"imageRef":     ref,
			"imageID":      imageDgst,
			"nameParseErr": err,
		}).Debug("filtering container's image")
		if err == nil {
			// If user provided a specific image name, exclude that image.
			name := reference.TagNameOnly(ref)
			delete(imagesToPrune, name.String())

			// Also exclude repo:tag image if repo:tag@sha256:digest reference was used.
			_, isDigested := name.(reference.Digested)
			tagged, isTagged := name.(reference.NamedTagged)
			if isDigested && isTagged {
				named, _ := reference.ParseNormalizedNamed(tagged.Name())
				namedTagged, _ := reference.WithTag(named, tagged.Tag())
				delete(imagesToPrune, namedTagged.String())
			}
		}
	}

	return usedDigests
}

// pruneAll deletes all images in the imagesToPrune map.
func (i *ImageService) pruneAll(ctx context.Context, imagesToPrune map[string]containerdimages.Image) (*image.PruneReport, error) {
	report := image.PruneReport{}

	ctx, span := tracing.StartSpan(ctx, "ImageService.pruneAll")
	span.SetAttributes(tracing.Attribute("count", len(imagesToPrune)))
	defer span.End()

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
