package containerd

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/content"
	cerrdefs "github.com/containerd/containerd/errdefs"
	cfilters "github.com/containerd/containerd/filters"
	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/platforms"
	"github.com/docker/distribution/reference"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/errdefs"
	"github.com/moby/buildkit/util/attestation"
	"github.com/opencontainers/go-digest"
	"github.com/opencontainers/image-spec/identity"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

var acceptedImageFilterTags = map[string]bool{
	"dangling":  true,
	"label":     true,
	"label!":    true,
	"before":    true,
	"since":     true,
	"reference": true,
}

// Images returns a filtered list of images.
//
// TODO(thaJeztah): sort the results by created (descending); see https://github.com/moby/moby/issues/43848
// TODO(thaJeztah): implement opts.ContainerCount (used for docker system df); see https://github.com/moby/moby/issues/43853
// TODO(thaJeztah): add labels to results; see https://github.com/moby/moby/issues/43852
// TODO(thaJeztah): verify behavior of `RepoDigests` and `RepoTags` for images without (untagged) or multiple tags; see https://github.com/moby/moby/issues/43861
// TODO(thaJeztah): verify "Size" vs "VirtualSize" in images; see https://github.com/moby/moby/issues/43862
func (i *ImageService) Images(ctx context.Context, opts types.ImageListOptions) ([]*types.ImageSummary, error) {
	if err := opts.Filters.Validate(acceptedImageFilterTags); err != nil {
		return nil, err
	}

	listFilters, filter, err := i.setupFilters(ctx, opts.Filters)
	if err != nil {
		return nil, err
	}

	imgs, err := i.client.ImageService().List(ctx, listFilters...)
	if err != nil {
		return nil, err
	}

	snapshotter := i.client.SnapshotService(i.snapshotter)
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

	var (
		summaries = make([]*types.ImageSummary, 0, len(imgs))
		root      []*[]digest.Digest
		layers    map[digest.Digest]int
	)
	if opts.SharedSize {
		root = make([]*[]digest.Digest, len(imgs))
		layers = make(map[digest.Digest]int)
	}

	contentStore := i.client.ContentStore()
	for _, img := range imgs {
		if !filter(img) {
			continue
		}

		err := images.Walk(ctx, images.HandlerFunc(func(ctx context.Context, desc v1.Descriptor) ([]v1.Descriptor, error) {
			if images.IsIndexType(desc.MediaType) {
				return images.Children(ctx, contentStore, desc)
			}

			if images.IsManifestType(desc.MediaType) {
				// Ignore buildkit attestation manifests
				// https://github.com/moby/buildkit/blob/v0.11.4/docs/attestations/attestation-storage.md
				// This would have also been caught by the isImageManifest call below, but it requires
				// an additional content read and deserialization of Manifest.
				if _, has := desc.Annotations[attestation.DockerAnnotationReferenceType]; has {
					return nil, nil
				}

				mfst, err := images.Manifest(ctx, contentStore, desc, platforms.All)
				if err != nil {
					if cerrdefs.IsNotFound(err) {
						return nil, nil
					}
					return nil, err
				}

				if !isImageManifest(mfst) {
					return nil, nil
				}

				platform, err := getManifestPlatform(ctx, contentStore, desc, mfst.Config)
				if err != nil {
					if cerrdefs.IsNotFound(err) {
						return nil, nil
					}
					return nil, err
				}

				pm := platforms.OnlyStrict(platform)
				available, _, _, missing, err := images.Check(ctx, contentStore, img.Target, pm)
				if err != nil {
					logrus.WithFields(logrus.Fields{
						logrus.ErrorKey: err,
						"platform":      platform,
						"image":         img.Target,
					}).Warn("checking availability of platform content failed")
					return nil, nil
				}
				if !available || len(missing) > 0 {
					return nil, nil
				}

				c8dImage := containerd.NewImageWithPlatform(i.client, img, pm)
				image, chainIDs, err := i.singlePlatformImage(ctx, contentStore, c8dImage)
				if err != nil {
					return nil, err
				}

				summaries = append(summaries, image)

				if opts.SharedSize {
					root = append(root, &chainIDs)
					for _, id := range chainIDs {
						layers[id] = layers[id] + 1
					}
				}
			}

			return nil, nil
		}), img.Target)

		if err != nil {
			return nil, err
		}

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

	return summaries, nil
}

func (i *ImageService) singlePlatformImage(ctx context.Context, contentStore content.Store, image containerd.Image) (*types.ImageSummary, []digest.Digest, error) {
	diffIDs, err := image.RootFS(ctx)
	if err != nil {
		return nil, nil, err
	}
	chainIDs := identity.ChainIDs(diffIDs)

	size, err := image.Size(ctx)
	if err != nil {
		return nil, nil, err
	}
	snapshotter := i.client.SnapshotService(i.snapshotter)
	sizeCache := make(map[digest.Digest]int64)

	snapshotSizeFn := func(d digest.Digest) (int64, error) {
		if s, ok := sizeCache[d]; ok {
			return s, nil
		}
		usage, err := snapshotter.Usage(ctx, d.String())
		if err != nil {
			if cerrdefs.IsNotFound(err) {
				return 0, nil
			}
			return 0, err
		}
		sizeCache[d] = usage.Size
		return usage.Size, nil
	}
	virtualSize, err := computeVirtualSize(chainIDs, snapshotSizeFn)
	if err != nil {
		return nil, nil, err
	}

	var repoTags, repoDigests []string
	rawImg := image.Metadata()
	target := rawImg.Target.Digest

	logger := logrus.WithFields(logrus.Fields{
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
		repoTags = append(repoTags, reference.TagNameOnly(ref).String())

		digested, err := reference.WithDigest(reference.TrimNamed(ref), target)
		if err != nil {
			logger.WithError(err).Error("failed to create digested reference")
		} else {
			repoDigests = append(repoDigests, digested.String())
		}
	}

	summary := &types.ImageSummary{
		ParentID:    "",
		ID:          target.String(),
		Created:     rawImg.CreatedAt.Unix(),
		RepoDigests: repoDigests,
		RepoTags:    repoTags,
		Size:        size,
		VirtualSize: virtualSize,
		// -1 indicates that the value has not been set (avoids ambiguity
		// between 0 (default) and "not set". We cannot use a pointer (nil)
		// for this, as the JSON representation uses "omitempty", which would
		// consider both "0" and "nil" to be "empty".
		SharedSize: -1,
		Containers: -1,
	}

	return summary, chainIDs, nil
}

type imageFilterFunc func(image images.Image) bool

// setupFilters constructs an imageFilterFunc from the given imageFilters.
//
// containerdListFilters is a slice of filters which should be passed to ImageService.List()
// filterFunc is a function that checks whether given image matches the filters.
// TODO(thaJeztah): reimplement filters using containerd filters: see https://github.com/moby/moby/issues/43845
func (i *ImageService) setupFilters(ctx context.Context, imageFilters filters.Args) (
	containerdListFilters []string, filterFunc imageFilterFunc, outErr error) {

	var fltrs []imageFilterFunc
	var listFilters []string
	err := imageFilters.WalkValues("before", func(value string) error {
		ref, err := reference.ParseDockerRef(value)
		if err != nil {
			return err
		}
		img, err := i.client.GetImage(ctx, ref.String())
		if img != nil {
			t := img.Metadata().CreatedAt
			fltrs = append(fltrs, func(image images.Image) bool {
				created := image.CreatedAt
				return created.Equal(t) || created.After(t)
			})
		}
		return err
	})
	if err != nil {
		return nil, nil, err
	}

	err = imageFilters.WalkValues("since", func(value string) error {
		ref, err := reference.ParseDockerRef(value)
		if err != nil {
			return err
		}
		img, err := i.client.GetImage(ctx, ref.String())
		if img != nil {
			t := img.Metadata().CreatedAt
			fltrs = append(fltrs, func(image images.Image) bool {
				created := image.CreatedAt
				return created.Equal(t) || created.Before(t)
			})
		}
		return err
	})
	if err != nil {
		return nil, nil, err
	}

	labelFn, err := setupLabelFilter(i.client.ContentStore(), imageFilters)
	if err != nil {
		return nil, nil, err
	}
	if labelFn != nil {
		fltrs = append(fltrs, labelFn)
	}

	if imageFilters.Contains("dangling") {
		danglingValue, err := imageFilters.GetBoolOrDefault("dangling", false)
		if err != nil {
			return nil, nil, err
		}
		fltrs = append(fltrs, func(image images.Image) bool {
			return danglingValue == isDanglingImage(image)
		})
	}

	err = imageFilters.WalkValues("reference", func(value string) error {
		ref, err := reference.ParseNormalizedNamed(value)
		if err != nil {
			return err
		}
		ref = reference.TagNameOnly(ref)
		listFilters = append(listFilters, "name=="+ref.String())
		return nil
	})
	if err != nil {
		return nil, nil, err
	}

	return listFilters, func(image images.Image) bool {
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
func setupLabelFilter(store content.Store, args filters.Args) (func(image images.Image) bool, error) {
	filter, err := buildContainerdKvFilter("label", args)
	if err != nil {
		return nil, err
	}
	return func(image images.Image) bool {
		ctx := context.TODO()

		errFoundConfig := errors.New("success, found matching config")
		err := images.Dispatch(ctx, presentChildrenHandler(store, images.HandlerFunc(
			func(ctx context.Context, desc v1.Descriptor) (subdescs []v1.Descriptor, err error) {
				if images.IsConfigType(desc.MediaType) {
					// Subset of ocispec.Image that only contains Labels
					var cfg struct {
						Config struct {
							Labels map[string]string `json:"Labels,omitempty"`
						} `json:"Config,omitempty"`
					}
					err := readConfig(ctx, store, desc, &cfg)
					if err != nil {
						return nil, err
					}

					match := filter.Match(cfilters.AdapterFunc(func(fieldpath []string) (string, bool) {
						if len(fieldpath) == 0 {
							return "", false
						}

						switch fieldpath[0] {
						case "label":
							key := strings.Join(fieldpath[1:], ".")
							value, has := cfg.Config.Labels[key]
							return value, has
						}
						return "", false
					}))

					if match {
						return nil, errFoundConfig
					}
					return nil, nil
				}

				return nil, nil
			})), nil, image.Target)

		if err == errFoundConfig {
			return true
		}
		if err != nil {
			logrus.WithFields(logrus.Fields{
				logrus.ErrorKey: err,
				"image":         image.Name,
			}).Error("failed to check image labels")
		}

		return false
	}, nil
}

// buildContainerdFilter builds containerd filter from the specified docker
// filter that use key=value syntax.
// It handles both normal and negated filter.
// Currently it only handles the conversion from "=" to "==" and
// "filter!=key=value" to "filter=key!=value".
func buildContainerdKvFilter(filter string, args filters.Args) (cfilters.Filter, error) {
	var fltrs []cfilters.Filter

	for _, fltrName := range []string{filter, filter + "!"} {
		values := args.Get(fltrName)

		op := "=="
		// Trim "!" from label! and set the operator to not-equals
		if strings.HasSuffix(fltrName, "!") {
			op = "!="
			fltrName = strings.TrimSuffix(fltrName, "!")
		}

		for _, l := range values {
			// This filter will check if the label exists
			// TODO: Handle not exists in "label!"
			f := fltrName + "." + l

			k, v, found := strings.Cut(l, "=")
			if found {
				op := op
				// Turn "label!=some.label=value" into "label.some.label!=value"
				// (this is to avoid it being changed into !==)
				if strings.HasSuffix(k, "!") {
					op = "!="
					k = strings.TrimSuffix(k, "!")
				}
				//
				f = fltrName + "." + k + op + v
			}

			fltr, err := cfilters.Parse(f)
			if err != nil {
				return nil, errdefs.InvalidParameter(err)
			}

			fltrs = append(fltrs, fltr)
		}
	}

	return cfilters.All(fltrs), nil
}

func computeVirtualSize(chainIDs []digest.Digest, sizeFn func(d digest.Digest) (int64, error)) (int64, error) {
	var virtualSize int64
	for _, chainID := range chainIDs {
		size, err := sizeFn(chainID)
		if err != nil {
			return virtualSize, err
		}
		virtualSize += size
	}
	return virtualSize, nil
}

func computeSharedSize(chainIDs []digest.Digest, layers map[digest.Digest]int, sizeFn func(d digest.Digest) (int64, error)) (int64, error) {
	var sharedSize int64
	for _, chainID := range chainIDs {
		if layers[chainID] == 1 {
			continue
		}
		size, err := sizeFn(chainID)
		if err != nil {
			return 0, err
		}
		sharedSize += size
	}
	return sharedSize, nil
}

// getManifestPlatform returns a platform specified by the manifest descriptor
// or reads it from its config.
func getManifestPlatform(ctx context.Context, store content.Provider, manifestDesc, configDesc v1.Descriptor) (v1.Platform, error) {
	var platform v1.Platform
	if manifestDesc.Platform != nil {
		platform = *manifestDesc.Platform
	} else {
		// Config is technically a v1.Image, but it has the same member as v1.Platform
		// which makes the v1.Platform a subset of Image so we can unmarshal directly.
		if err := readConfig(ctx, store, configDesc, &platform); err != nil {
			return platform, err
		}
	}
	return platforms.Normalize(platform), nil
}

// isImageManifests returns true if the manifest has any layer that is a known image layer.
// Some manifests use the image media type for compatibility, even if they are not a real image.
func isImageManifest(mfst v1.Manifest) bool {
	for _, l := range mfst.Layers {
		if images.IsLayerType(l.MediaType) {
			return true
		}
	}
	return false
}

// readConfig reads content pointed by the descriptor and unmarshals it into a specified output.
func readConfig(ctx context.Context, store content.Provider, desc v1.Descriptor, out interface{}) error {
	data, err := content.ReadBlob(ctx, store, desc)
	if err != nil {
		return errors.Wrapf(err, "failed to read config content")
	}
	err = json.Unmarshal(data, out)
	if err != nil {
		return errors.Wrapf(err, "could not deserialize image config")
	}

	return nil
}
