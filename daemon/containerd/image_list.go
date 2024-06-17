package containerd

import (
	"context"
	"encoding/json"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/labels"
	"github.com/containerd/containerd/platforms"
	"github.com/containerd/containerd/snapshots"
	cerrdefs "github.com/containerd/errdefs"
	"github.com/containerd/log"
	"github.com/distribution/reference"
	"github.com/docker/docker/api/types/backend"
	"github.com/docker/docker/api/types/filters"
	imagetypes "github.com/docker/docker/api/types/image"
	timetypes "github.com/docker/docker/api/types/time"
	"github.com/docker/docker/container"
	"github.com/docker/docker/errdefs"
	dockerspec "github.com/moby/docker-image-spec/specs-go/v1"
	"github.com/opencontainers/go-digest"
	"github.com/opencontainers/image-spec/identity"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
)

// Subset of ocispec.Image that only contains Labels
type configLabels struct {
	// Created is the combined date and time at which the image was created, formatted as defined by RFC 3339, section 5.6.
	Created *time.Time `json:"created,omitempty"`

	Config struct {
		Labels map[string]string `json:"Labels,omitempty"`
	} `json:"config,omitempty"`
}

var acceptedImageFilterTags = map[string]bool{
	"dangling":  true,
	"label":     true,
	"label!":    true,
	"before":    true,
	"since":     true,
	"reference": true,
	"until":     true,
}

// byCreated is a temporary type used to sort a list of images by creation
// time.
type byCreated []*imagetypes.Summary

func (r byCreated) Len() int           { return len(r) }
func (r byCreated) Swap(i, j int)      { r[i], r[j] = r[j], r[i] }
func (r byCreated) Less(i, j int) bool { return r[i].Created < r[j].Created }

// Images returns a filtered list of images.
//
// TODO(thaJeztah): verify behavior of `RepoDigests` and `RepoTags` for images without (untagged) or multiple tags; see https://github.com/moby/moby/issues/43861
// TODO(thaJeztah): verify "Size" vs "VirtualSize" in images; see https://github.com/moby/moby/issues/43862
func (i *ImageService) Images(ctx context.Context, opts imagetypes.ListOptions) ([]*imagetypes.Summary, error) {
	if err := opts.Filters.Validate(acceptedImageFilterTags); err != nil {
		return nil, err
	}

	filter, err := i.setupFilters(ctx, opts.Filters)
	if err != nil {
		return nil, err
	}

	imgs, err := i.images.List(ctx)
	if err != nil {
		return nil, err
	}

	// TODO(thaJeztah): do we need to take multiple snapshotters into account? See https://github.com/moby/moby/issues/45273
	snapshotter := i.snapshotterService(i.snapshotter)
	sizeCache := make(map[digest.Digest]int64)
	snapshotSizeFn := func(d digest.Digest) (int64, error) {
		if s, ok := sizeCache[d]; ok {
			return s, nil
		}
		usage, err := snapshotter.Usage(ctx, d.String())
		if err != nil {
			return 0, err
		}
		sizeCache[d] = usage.Size
		return usage.Size, nil
	}

	uniqueImages := map[digest.Digest]images.Image{}
	tagsByDigest := map[digest.Digest][]string{}
	intermediateImages := map[digest.Digest]struct{}{}

	hideIntermediate := !opts.All
	if hideIntermediate {
		for _, img := range imgs {
			parent, ok := img.Labels[imageLabelClassicBuilderParent]
			if ok && parent != "" {
				dgst, err := digest.Parse(parent)
				if err != nil {
					log.G(ctx).WithFields(log.Fields{
						"error": err,
						"value": parent,
					}).Warnf("invalid %s label value", imageLabelClassicBuilderParent)
				}
				intermediateImages[dgst] = struct{}{}
			}
		}
	}

	// TODO: Allow platform override?
	platformMatcher := matchAllWithPreference(platforms.Default())

	for _, img := range imgs {
		isDangling := isDanglingImage(img)

		if hideIntermediate && isDangling {
			if _, ok := intermediateImages[img.Target.Digest]; ok {
				continue
			}
		}

		if !filter(img) {
			continue
		}

		dgst := img.Target.Digest
		uniqueImages[dgst] = img

		if isDangling {
			continue
		}

		ref, err := reference.ParseNormalizedNamed(img.Name)
		if err != nil {
			continue
		}
		tagsByDigest[dgst] = append(tagsByDigest[dgst], reference.FamiliarString(ref))
	}

	resultsMut := sync.Mutex{}
	eg, egCtx := errgroup.WithContext(ctx)
	eg.SetLimit(runtime.NumCPU() * 2)

	var (
		summaries = make([]*imagetypes.Summary, 0, len(imgs))
		root      []*[]digest.Digest
		layers    map[digest.Digest]int
	)
	if opts.SharedSize {
		root = make([]*[]digest.Digest, 0, len(imgs))
		layers = make(map[digest.Digest]int)
	}

	for _, img := range uniqueImages {
		img := img
		eg.Go(func() error {
			image, allChainsIDs, err := i.imageSummary(egCtx, img, platformMatcher, opts, tagsByDigest)
			if err != nil {
				return err
			}
			// No error, but image should be skipped.
			if image == nil {
				return nil
			}

			resultsMut.Lock()
			summaries = append(summaries, image)

			if opts.SharedSize {
				root = append(root, &allChainsIDs)
				for _, id := range allChainsIDs {
					layers[id] = layers[id] + 1
				}
			}
			resultsMut.Unlock()
			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		return nil, err
	}

	if opts.SharedSize {
		for n, chainIDs := range root {
			sharedSize, err := computeSharedSize(*chainIDs, layers, snapshotSizeFn)
			if err != nil {
				return nil, err
			}
			summaries[n].SharedSize = sharedSize
		}
	}

	sort.Sort(sort.Reverse(byCreated(summaries)))

	return summaries, nil
}

// imageSummary returns a summary of the image, including the total size of the image and all its platforms.
// It also returns the chainIDs of all the layers of the image (including all its platforms).
// All return values will be nil if the image should be skipped.
func (i *ImageService) imageSummary(ctx context.Context, img images.Image, platformMatcher platforms.MatchComparer,
	opts imagetypes.ListOptions, tagsByDigest map[digest.Digest][]string,
) (_ *imagetypes.Summary, allChainIDs []digest.Digest, _ error) {

	// Total size of the image including all its platform
	var totalSize int64

	// ChainIDs of all the layers of the image (including all its platform)
	var allChainsIDs []digest.Digest

	// Count of containers using the image
	var containersCount int64

	// Single platform image manifest preferred by the platform matcher
	var best *ImageManifest
	var bestPlatform ocispec.Platform

	err := i.walkImageManifests(ctx, img, func(img *ImageManifest) error {
		if isPseudo, err := img.IsPseudoImage(ctx); isPseudo || err != nil {
			return nil
		}

		available, err := img.CheckContentAvailable(ctx)
		if err != nil {
			log.G(ctx).WithFields(log.Fields{
				"error":    err,
				"manifest": img.Target(),
				"image":    img.Name(),
			}).Warn("checking availability of platform specific manifest failed")
			return nil
		}

		if !available {
			return nil
		}

		conf, err := img.Config(ctx)
		if err != nil {
			return err
		}

		var dockerImage dockerspec.DockerOCIImage
		if err := readConfig(ctx, i.content, conf, &dockerImage); err != nil {
			return err
		}

		target := img.Target()

		diffIDs, err := img.RootFS(ctx)
		if err != nil {
			return err
		}

		chainIDs := identity.ChainIDs(diffIDs)

		ts, _, err := i.singlePlatformSize(ctx, img)
		if err != nil {
			return err
		}

		totalSize += ts
		allChainsIDs = append(allChainsIDs, chainIDs...)

		if opts.ContainerCount {
			i.containers.ApplyAll(func(c *container.Container) {
				if c.ImageManifest != nil && c.ImageManifest.Digest == target.Digest {
					containersCount++
				}
			})
		}

		var platform ocispec.Platform
		if target.Platform != nil {
			platform = *target.Platform
		} else {
			platform = dockerImage.Platform
		}

		// Filter out platforms that don't match the requested platform.  Do it
		// after the size, container count and chainIDs are summed up to have
		// the single combined entry still represent the whole multi-platform
		// image.
		if !platformMatcher.Match(platform) {
			return nil
		}

		if best == nil || platformMatcher.Less(platform, bestPlatform) {
			best = img
			bestPlatform = platform
		}

		return nil
	})
	if err != nil {
		if errors.Is(err, errNotManifestOrIndex) {
			log.G(ctx).WithFields(log.Fields{
				"error": err,
				"image": img.Name,
			}).Warn("unexpected image target (neither a manifest nor index)")
			return nil, nil, nil
		}
		return nil, nil, err
	}

	if best == nil {
		// TODO we should probably show *something* for images we've pulled
		// but are 100% shallow or an empty manifest list/index
		// ("tianon/scratch:index" is an empty example image index and
		// "tianon/scratch:list" is an empty example manifest list)
		return nil, nil, nil
	}

	image, err := i.singlePlatformImage(ctx, i.content, tagsByDigest[best.RealTarget.Digest], best)
	if err != nil {
		return nil, nil, err
	}
	image.Size = totalSize

	if opts.ContainerCount {
		image.Containers = containersCount
	}
	return image, allChainsIDs, nil
}

func (i *ImageService) singlePlatformSize(ctx context.Context, imgMfst *ImageManifest) (totalSize int64, contentSize int64, _ error) {
	// TODO(thaJeztah): do we need to take multiple snapshotters into account? See https://github.com/moby/moby/issues/45273
	snapshotter := i.snapshotterService(i.snapshotter)

	diffIDs, err := imgMfst.RootFS(ctx)
	if err != nil {
		return -1, -1, errors.Wrapf(err, "failed to get rootfs of image %s", imgMfst.Name())
	}

	imageSnapshotID := identity.ChainID(diffIDs).String()
	unpackedUsage, err := calculateSnapshotTotalUsage(ctx, snapshotter, imageSnapshotID)
	if err != nil {
		if !cerrdefs.IsNotFound(err) {
			log.G(ctx).WithError(err).WithFields(log.Fields{
				"image":      imgMfst.Name(),
				"snapshotID": imageSnapshotID,
			}).Warn("failed to calculate unpacked size of image")
		}
		unpackedUsage = snapshots.Usage{Size: 0}
	}

	contentSize, err = imgMfst.Size(ctx)
	if err != nil {
		return -1, -1, err
	}

	// totalSize is the size of the image's packed layers and snapshots
	// (unpacked layers) combined.
	totalSize = contentSize + unpackedUsage.Size
	return totalSize, contentSize, nil
}

func (i *ImageService) singlePlatformImage(ctx context.Context, contentStore content.Store, repoTags []string, imageManifest *ImageManifest) (*imagetypes.Summary, error) {
	var repoDigests []string
	rawImg := imageManifest.Metadata()
	target := rawImg.Target.Digest

	logger := log.G(ctx).WithFields(log.Fields{
		"name":   rawImg.Name,
		"digest": target,
	})

	ref, err := reference.ParseNamed(rawImg.Name)
	if err != nil {
		// If the image has unexpected name format (not a Named reference or a dangling image)
		// add the offending name to RepoTags but also log an error to make it clear to the
		// administrator that this is unexpected.
		// TODO: Reconsider when containerd is more strict on image names, see:
		//       https://github.com/containerd/containerd/issues/7986
		if !isDanglingImage(rawImg) {
			logger.WithError(err).Error("failed to parse image name as reference")
			repoTags = append(repoTags, rawImg.Name)
		}
	} else {
		digested, err := reference.WithDigest(reference.TrimNamed(ref), target)
		if err != nil {
			logger.WithError(err).Error("failed to create digested reference")
		} else {
			repoDigests = append(repoDigests, reference.FamiliarString(digested))
		}
	}

	cfgDesc, err := imageManifest.Image.Config(ctx)
	if err != nil {
		return nil, err
	}
	var cfg configLabels
	if err := readConfig(ctx, contentStore, cfgDesc, &cfg); err != nil {
		return nil, err
	}

	totalSize, _, err := i.singlePlatformSize(ctx, imageManifest)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to calculate size of image %s", imageManifest.Name())
	}

	summary := &imagetypes.Summary{
		ParentID:    rawImg.Labels[imageLabelClassicBuilderParent],
		ID:          target.String(),
		RepoDigests: repoDigests,
		RepoTags:    repoTags,
		Size:        totalSize,
		Labels:      cfg.Config.Labels,
		// -1 indicates that the value has not been set (avoids ambiguity
		// between 0 (default) and "not set". We cannot use a pointer (nil)
		// for this, as the JSON representation uses "omitempty", which would
		// consider both "0" and "nil" to be "empty".
		SharedSize: -1,
		Containers: -1,
	}
	if cfg.Created != nil {
		summary.Created = cfg.Created.Unix()
	}

	return summary, nil
}

type imageFilterFunc func(image images.Image) bool

// setupFilters constructs an imageFilterFunc from the given imageFilters.
//
// filterFunc is a function that checks whether given image matches the filters.
// TODO(thaJeztah): reimplement filters using containerd filters if possible: see https://github.com/moby/moby/issues/43845
func (i *ImageService) setupFilters(ctx context.Context, imageFilters filters.Args) (filterFunc imageFilterFunc, outErr error) {
	var fltrs []imageFilterFunc
	err := imageFilters.WalkValues("before", func(value string) error {
		img, err := i.GetImage(ctx, value, backend.GetImageOpts{})
		if err != nil {
			return err
		}
		if img != nil && img.Created != nil {
			fltrs = append(fltrs, func(candidate images.Image) bool {
				cand, err := i.GetImage(ctx, candidate.Name, backend.GetImageOpts{})
				if err != nil {
					return false
				}
				return cand.Created != nil && cand.Created.Before(*img.Created)
			})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	err = imageFilters.WalkValues("since", func(value string) error {
		img, err := i.GetImage(ctx, value, backend.GetImageOpts{})
		if err != nil {
			return err
		}
		if img != nil && img.Created != nil {
			fltrs = append(fltrs, func(candidate images.Image) bool {
				cand, err := i.GetImage(ctx, candidate.Name, backend.GetImageOpts{})
				if err != nil {
					return false
				}
				return cand.Created != nil && cand.Created.After(*img.Created)
			})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	err = imageFilters.WalkValues("until", func(value string) error {
		ts, err := timetypes.GetTimestamp(value, time.Now())
		if err != nil {
			return err
		}
		seconds, nanoseconds, err := timetypes.ParseTimestamps(ts, 0)
		if err != nil {
			return err
		}
		until := time.Unix(seconds, nanoseconds)

		fltrs = append(fltrs, func(image images.Image) bool {
			created := image.CreatedAt
			return created.Before(until)
		})
		return err
	})
	if err != nil {
		return nil, err
	}

	labelFn, err := setupLabelFilter(ctx, i.content, imageFilters)
	if err != nil {
		return nil, err
	}
	if labelFn != nil {
		fltrs = append(fltrs, labelFn)
	}

	if imageFilters.Contains("dangling") {
		danglingValue, err := imageFilters.GetBoolOrDefault("dangling", false)
		if err != nil {
			return nil, err
		}
		fltrs = append(fltrs, func(image images.Image) bool {
			return danglingValue == isDanglingImage(image)
		})
	}

	if refs := imageFilters.Get("reference"); len(refs) != 0 {
		fltrs = append(fltrs, func(image images.Image) bool {
			ref, err := reference.ParseNormalizedNamed(image.Name)
			if err != nil {
				return false
			}
			for _, value := range refs {
				found, err := reference.FamiliarMatch(value, ref)
				if err != nil {
					return false
				}
				if found {
					return found
				}
			}
			return false
		})
	}

	return func(image images.Image) bool {
		for _, filter := range fltrs {
			if !filter(image) {
				return false
			}
		}
		return true
	}, nil
}

// setupLabelFilter parses filter args for "label" and "label!" and returns a
// filter func which will check if any image config from the given image has
// labels that match given predicates.
func setupLabelFilter(ctx context.Context, store content.Store, fltrs filters.Args) (func(image images.Image) bool, error) {
	type labelCheck struct {
		key        string
		value      string
		onlyExists bool
		negate     bool
	}

	var checks []labelCheck
	for _, fltrName := range []string{"label", "label!"} {
		for _, l := range fltrs.Get(fltrName) {
			k, v, found := strings.Cut(l, "=")
			err := labels.Validate(k, v)
			if err != nil {
				return nil, err
			}

			negate := strings.HasSuffix(fltrName, "!")

			// If filter value is key!=value then flip the above.
			if strings.HasSuffix(k, "!") {
				k = strings.TrimSuffix(k, "!")
				negate = !negate
			}

			checks = append(checks, labelCheck{
				key:        k,
				value:      v,
				onlyExists: !found,
				negate:     negate,
			})
		}
	}

	if len(checks) == 0 {
		return nil, nil
	}

	return func(image images.Image) bool {
		// This is not an error, but a signal to Dispatch that it should stop
		// processing more content (otherwise it will run for all children).
		// It will be returned once a matching config is found.
		errFoundConfig := errors.New("success, found matching config")

		err := images.Dispatch(ctx, presentChildrenHandler(store, images.HandlerFunc(func(ctx context.Context, desc ocispec.Descriptor) (subdescs []ocispec.Descriptor, err error) {
			if !images.IsConfigType(desc.MediaType) {
				return nil, nil
			}
			var cfg configLabels
			if err := readConfig(ctx, store, desc, &cfg); err != nil {
				if errdefs.IsNotFound(err) {
					return nil, nil
				}
				return nil, err
			}

			for _, check := range checks {
				value, exists := cfg.Config.Labels[check.key]

				if check.onlyExists {
					// label! given without value, check if doesn't exist
					if check.negate {
						// Label exists, config doesn't match
						if exists {
							return nil, nil
						}
					} else {
						// Label should exist
						if !exists {
							// Label doesn't exist, config doesn't match
							return nil, nil
						}
					}
					continue
				} else if !exists {
					// We are checking value and label doesn't exist.
					return nil, nil
				}

				valueEquals := value == check.value
				if valueEquals == check.negate {
					return nil, nil
				}
			}

			// This config matches the filter so we need to shop this image, stop dispatch.
			return nil, errFoundConfig
		})), nil, image.Target)

		if err == errFoundConfig {
			return true
		}
		if err != nil {
			log.G(ctx).WithFields(log.Fields{
				"error":  err,
				"image":  image.Name,
				"checks": checks,
			}).Error("failed to check image labels")
		}

		return false
	}, nil
}

func computeSharedSize(chainIDs []digest.Digest, layers map[digest.Digest]int, sizeFn func(d digest.Digest) (int64, error)) (int64, error) {
	var sharedSize int64
	for _, chainID := range chainIDs {
		if layers[chainID] == 1 {
			continue
		}
		size, err := sizeFn(chainID)
		if err != nil {
			// Several images might share the same layer and neither of them
			// might be unpacked (for example if it's a non-host platform).
			if cerrdefs.IsNotFound(err) {
				continue
			}
			return 0, err
		}
		sharedSize += size
	}
	return sharedSize, nil
}

// readConfig reads content pointed by the descriptor and unmarshals it into a specified output.
func readConfig(ctx context.Context, store content.Provider, desc ocispec.Descriptor, out interface{}) error {
	data, err := content.ReadBlob(ctx, store, desc)
	if err != nil {
		err = errors.Wrapf(err, "failed to read config content")
		if cerrdefs.IsNotFound(err) {
			return errdefs.NotFound(err)
		}
		return err
	}

	err = json.Unmarshal(data, out)
	if err != nil {
		err = errors.Wrapf(err, "could not deserialize image config")
		if cerrdefs.IsNotFound(err) {
			return errdefs.NotFound(err)
		}
		return err
	}

	return nil
}
