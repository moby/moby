package containerd

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	cerrdefs "github.com/containerd/containerd/errdefs"
	containerdimages "github.com/containerd/containerd/images"
	"github.com/containerd/containerd/platforms"
	"github.com/containerd/log"
	"github.com/distribution/reference"
	"github.com/docker/docker/api/types/backend"
	"github.com/docker/docker/daemon/images"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/image"
	imagespec "github.com/moby/docker-image-spec/specs-go/v1"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"golang.org/x/sync/semaphore"
)

var truncatedID = regexp.MustCompile(`^(sha256:)?([a-f0-9]{4,64})$`)

var errInconsistentData error = errors.New("consistency error: data changed during operation, retry")

// GetImage returns an image corresponding to the image referred to by refOrID.
func (i *ImageService) GetImage(ctx context.Context, refOrID string, options backend.GetImageOpts) (*image.Image, error) {
	desc, err := i.resolveImage(ctx, refOrID)
	if err != nil {
		return nil, err
	}

	platform := matchAllWithPreference(platforms.Default())
	if options.Platform != nil {
		platform = platforms.OnlyStrict(*options.Platform)
	}

	presentImages, err := i.presentImages(ctx, desc, refOrID, platform)
	if err != nil {
		return nil, err
	}
	ociImage := presentImages[0]

	img := dockerOciImageToDockerImagePartial(image.ID(desc.Target.Digest), ociImage)

	parent, err := i.getImageLabelByDigest(ctx, desc.Target.Digest, imageLabelClassicBuilderParent)
	if err != nil {
		log.G(ctx).WithError(err).Warn("failed to determine Parent property")
	} else {
		img.Parent = image.ID(parent)
	}

	if options.Details {
		lastUpdated := time.Unix(0, 0)
		size, err := i.size(ctx, desc.Target, platform)
		if err != nil {
			return nil, err
		}

		tagged, err := i.images.List(ctx, "target.digest=="+desc.Target.Digest.String())
		if err != nil {
			return nil, err
		}

		// Usually each image will result in 2 references (named and digested).
		refs := make([]reference.Named, 0, len(tagged)*2)
		for _, i := range tagged {
			if i.UpdatedAt.After(lastUpdated) {
				lastUpdated = i.UpdatedAt
			}
			if isDanglingImage(i) {
				if len(tagged) > 1 {
					// This is unexpected - dangling image should be deleted
					// as soon as another image with the same target is created.
					// Log a warning, but don't error out the whole operation.
					log.G(ctx).WithField("refs", tagged).Warn("multiple images have the same target, but one of them is still dangling")
				}
				continue
			}

			name, err := reference.ParseNamed(i.Name)
			if err != nil {
				// This is inconsistent with `docker image ls` which will
				// still include the malformed name in RepoTags.
				log.G(ctx).WithField("name", name).WithError(err).Error("failed to parse image name as reference")
				continue
			}
			refs = append(refs, name)

			if _, ok := name.(reference.Digested); ok {
				// Image name already contains a digest, so no need to create a digested reference.
				continue
			}

			digested, err := reference.WithDigest(reference.TrimNamed(name), desc.Target.Digest)
			if err != nil {
				// This could only happen if digest is invalid, but considering that
				// we get it from the Descriptor it's highly unlikely.
				// Log error just in case.
				log.G(ctx).WithError(err).Error("failed to create digested reference")
				continue
			}
			refs = append(refs, digested)
		}

		img.Details = &image.Details{
			References:  refs,
			Size:        size,
			Metadata:    nil,
			Driver:      i.snapshotter,
			LastUpdated: lastUpdated,
		}
	}

	return img, nil
}

// presentImages returns the images that are present in the content store,
// manifests without a config are ignored.
// The images are filtered and sorted by platform preference.
func (i *ImageService) presentImages(ctx context.Context, desc containerdimages.Image, refOrID string, platform platforms.MatchComparer) ([]imagespec.DockerOCIImage, error) {
	var presentImages []imagespec.DockerOCIImage
	err := i.walkImageManifests(ctx, desc, func(img *ImageManifest) error {
		conf, err := img.Config(ctx)
		if err != nil {
			if cerrdefs.IsNotFound(err) {
				log.G(ctx).WithFields(log.Fields{
					"manifestDescriptor": img.Target(),
				}).Debug("manifest was present, but accessing its config failed, ignoring")
				return nil
			}
			return errdefs.System(fmt.Errorf("failed to get config descriptor: %w", err))
		}

		var ociimage imagespec.DockerOCIImage
		if err := readConfig(ctx, i.content, conf, &ociimage); err != nil {
			if errdefs.IsNotFound(err) {
				log.G(ctx).WithFields(log.Fields{
					"manifestDescriptor": img.Target(),
					"configDescriptor":   conf,
				}).Debug("manifest present, but its config is missing, ignoring")
				return nil
			}
			return errdefs.System(fmt.Errorf("failed to read config of the manifest %v: %w", img.Target().Digest, err))
		}

		if platform.Match(ociimage.Platform) {
			presentImages = append(presentImages, ociimage)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}
	if len(presentImages) == 0 {
		ref, _ := reference.ParseAnyReference(refOrID)
		return nil, images.ErrImageDoesNotExist{Ref: ref}
	}

	sort.SliceStable(presentImages, func(i, j int) bool {
		return platform.Less(presentImages[i].Platform, presentImages[j].Platform)
	})

	return presentImages, nil
}

func (i *ImageService) GetImageManifest(ctx context.Context, refOrID string, options backend.GetImageOpts) (*ocispec.Descriptor, error) {
	platform := matchAllWithPreference(platforms.Default())
	if options.Platform != nil {
		platform = platforms.Only(*options.Platform)
	}

	cs := i.client.ContentStore()

	img, err := i.resolveImage(ctx, refOrID)
	if err != nil {
		return nil, err
	}

	desc := img.Target
	if containerdimages.IsManifestType(desc.MediaType) {
		plat := desc.Platform
		if plat == nil {
			config, err := img.Config(ctx, cs, platform)
			if err != nil {
				return nil, err
			}
			var configPlatform ocispec.Platform
			if err := readConfig(ctx, cs, config, &configPlatform); err != nil {
				return nil, err
			}

			plat = &configPlatform
		}

		if options.Platform != nil {
			if plat == nil {
				return nil, errdefs.NotFound(errors.Errorf("image with reference %s was found but does not match the specified platform: wanted %s, actual: nil", refOrID, platforms.Format(*options.Platform)))
			} else if !platform.Match(*plat) {
				return nil, errdefs.NotFound(errors.Errorf("image with reference %s was found but does not match the specified platform: wanted %s, actual: %s", refOrID, platforms.Format(*options.Platform), platforms.Format(*plat)))
			}
		}

		return &desc, nil
	}

	if containerdimages.IsIndexType(desc.MediaType) {
		childManifests, err := containerdimages.LimitManifests(containerdimages.ChildrenHandler(cs), platform, 1)(ctx, desc)
		if err != nil {
			if cerrdefs.IsNotFound(err) {
				return nil, errdefs.NotFound(err)
			}
			return nil, errdefs.System(err)
		}

		// len(childManifests) == 1 since we requested 1 and if none
		// were found LimitManifests would have thrown an error
		if !containerdimages.IsManifestType(childManifests[0].MediaType) {
			return nil, errdefs.NotFound(fmt.Errorf("manifest has incorrect mediatype: %s", childManifests[0].MediaType))
		}

		return &childManifests[0], nil
	}

	return nil, errdefs.NotFound(errors.New("failed to find manifest"))
}

// size returns the total size of the image's packed resources.
func (i *ImageService) size(ctx context.Context, desc ocispec.Descriptor, platform platforms.MatchComparer) (int64, error) {
	var size int64

	cs := i.client.ContentStore()
	handler := containerdimages.LimitManifests(containerdimages.ChildrenHandler(cs), platform, 1)

	var wh containerdimages.HandlerFunc = func(ctx context.Context, desc ocispec.Descriptor) ([]ocispec.Descriptor, error) {
		children, err := handler(ctx, desc)
		if err != nil {
			if !cerrdefs.IsNotFound(err) {
				return nil, err
			}
		}

		atomic.AddInt64(&size, desc.Size)

		return children, nil
	}

	l := semaphore.NewWeighted(3)
	if err := containerdimages.Dispatch(ctx, wh, l, desc); err != nil {
		return 0, err
	}

	return size, nil
}

// resolveDescriptor searches for a descriptor based on the given
// reference or identifier. Returns the descriptor of
// the image, which could be a manifest list, manifest, or config.
func (i *ImageService) resolveDescriptor(ctx context.Context, refOrID string) (ocispec.Descriptor, error) {
	img, err := i.resolveImage(ctx, refOrID)
	if err != nil {
		return ocispec.Descriptor{}, err
	}

	return img.Target, nil
}

func (i *ImageService) resolveImage(ctx context.Context, refOrID string) (containerdimages.Image, error) {
	parsed, err := reference.ParseAnyReference(refOrID)
	if err != nil {
		return containerdimages.Image{}, errdefs.InvalidParameter(err)
	}

	digested, ok := parsed.(reference.Digested)
	if ok {
		imgs, err := i.images.List(ctx, "target.digest=="+digested.Digest().String())
		if err != nil {
			return containerdimages.Image{}, errors.Wrap(err, "failed to lookup digest")
		}
		if len(imgs) == 0 {
			return containerdimages.Image{}, images.ErrImageDoesNotExist{Ref: parsed}
		}

		// If reference is both Named and Digested, make sure we don't match
		// images with a different repository even if digest matches.
		// For example, busybox@sha256:abcdef..., shouldn't match asdf@sha256:abcdef...
		if parsedNamed, ok := parsed.(reference.Named); ok {
			for _, img := range imgs {
				imgNamed, err := reference.ParseNormalizedNamed(img.Name)
				if err != nil {
					log.G(ctx).WithError(err).WithField("image", img.Name).Warn("image with invalid name encountered")
					continue
				}

				if parsedNamed.Name() == imgNamed.Name() {
					return img, nil
				}
			}
			return containerdimages.Image{}, images.ErrImageDoesNotExist{Ref: parsed}
		}

		return imgs[0], nil
	}

	ref := reference.TagNameOnly(parsed.(reference.Named)).String()
	img, err := i.images.Get(ctx, ref)
	if err == nil {
		return img, nil
	} else {
		// TODO(containerd): error translation can use common function
		if !cerrdefs.IsNotFound(err) {
			return containerdimages.Image{}, err
		}
	}

	// If the identifier could be a short ID, attempt to match
	if truncatedID.MatchString(refOrID) {
		idWithoutAlgo := strings.TrimPrefix(refOrID, "sha256:")
		filters := []string{
			fmt.Sprintf("name==%q", ref), // Or it could just look like one.
			"target.digest~=" + strconv.Quote(fmt.Sprintf(`^sha256:%s[0-9a-fA-F]{%d}$`, regexp.QuoteMeta(idWithoutAlgo), 64-len(idWithoutAlgo))),
		}
		imgs, err := i.images.List(ctx, filters...)
		if err != nil {
			return containerdimages.Image{}, err
		}

		if len(imgs) == 0 {
			return containerdimages.Image{}, images.ErrImageDoesNotExist{Ref: parsed}
		}
		if len(imgs) > 1 {
			digests := map[digest.Digest]struct{}{}
			for _, img := range imgs {
				if img.Name == ref {
					return img, nil
				}
				digests[img.Target.Digest] = struct{}{}
			}

			if len(digests) > 1 {
				return containerdimages.Image{}, errdefs.NotFound(errors.New("ambiguous reference"))
			}
		}

		return imgs[0], nil
	}

	return containerdimages.Image{}, images.ErrImageDoesNotExist{Ref: parsed}
}

// getAllImagesWithRepository returns a slice of images which name is a reference
// pointing to the same repository as the given reference.
func (i *ImageService) getAllImagesWithRepository(ctx context.Context, ref reference.Named) ([]containerdimages.Image, error) {
	nameFilter := "^" + regexp.QuoteMeta(ref.Name()) + ":" + reference.TagRegexp.String() + "$"
	return i.client.ImageService().List(ctx, "name~="+strconv.Quote(nameFilter))
}

func imageFamiliarName(img containerdimages.Image) string {
	if isDanglingImage(img) {
		return img.Target.Digest.String()
	}

	if ref, err := reference.ParseNamed(img.Name); err == nil {
		return reference.FamiliarString(ref)
	}
	return img.Name
}

// getImageLabelByDigest will return the value of the label for images
// targeting the specified digest.
// If images have different values, an errdefs.Conflict error will be returned.
func (i *ImageService) getImageLabelByDigest(ctx context.Context, target digest.Digest, labelKey string) (string, error) {
	imgs, err := i.client.ImageService().List(ctx, "target.digest=="+target.String()+",labels."+labelKey)
	if err != nil {
		return "", errdefs.System(err)
	}

	var value string
	for _, img := range imgs {
		if v, ok := img.Labels[labelKey]; ok {
			if value != "" && value != v {
				return value, errdefs.Conflict(fmt.Errorf("conflicting label value %q and %q", value, v))
			}
			value = v
		}
	}

	return value, nil
}

func convertError(err error) error {
	// TODO: Convert containerd error to Docker error
	return err
}

// resolveAllReferences resolves the reference name or ID to an image and returns all the images with
// the same target.
//
// Returns:
//
// 1: *(github.com/containerd/containerd/images).Image
//
//	An image match from the image store with the provided refOrID
//
// 2: [](github.com/containerd/containerd/images).Image
//
//	List of all images with the same target that matches the refOrID. If the first argument is
//	non-nil, the image list will all have the same target as the matched image. If the first
//	argument is nil but the list is non-empty, this value is a list of all the images with a
//	target that matches the digest provided in the refOrID, but none are an image name match
//	to refOrID.
//
// 3: error
//
//	An error looking up refOrID or no images found with matching name or target. Note that the first
//	argument may be nil with a nil error if the second argument is non-empty.
func (i *ImageService) resolveAllReferences(ctx context.Context, refOrID string) (*containerdimages.Image, []containerdimages.Image, error) {
	parsed, err := reference.ParseAnyReference(refOrID)
	if err != nil {
		return nil, nil, errdefs.InvalidParameter(err)
	}
	var dgst digest.Digest
	var img *containerdimages.Image

	if truncatedID.MatchString(refOrID) {
		if d, ok := parsed.(reference.Digested); ok {
			if cimg, err := i.images.Get(ctx, d.String()); err == nil {
				img = &cimg
				dgst = d.Digest()
				if cimg.Target.Digest != dgst {
					// Ambiguous image reference, use reference name
					log.G(ctx).WithField("image", refOrID).WithField("target", cimg.Target.Digest).Warn("digest reference points to image with a different digest")
					dgst = cimg.Target.Digest
				}
			} else if !cerrdefs.IsNotFound(err) {
				return nil, nil, convertError(err)
			} else {
				dgst = d.Digest()
			}
		} else {
			idWithoutAlgo := strings.TrimPrefix(refOrID, "sha256:")
			name := reference.TagNameOnly(parsed.(reference.Named)).String()
			filters := []string{
				fmt.Sprintf("name==%q", name), // Or it could just look like one.
				"target.digest~=" + strconv.Quote(fmt.Sprintf(`^sha256:%s[0-9a-fA-F]{%d}$`, regexp.QuoteMeta(idWithoutAlgo), 64-len(idWithoutAlgo))),
			}
			imgs, err := i.images.List(ctx, filters...)
			if err != nil {
				return nil, nil, convertError(err)
			}

			if len(imgs) == 0 {
				return nil, nil, images.ErrImageDoesNotExist{Ref: parsed}
			}

			for _, limg := range imgs {
				if limg.Name == name {
					copyImg := limg
					img = &copyImg
				}
				if dgst != "" {
					if limg.Target.Digest != dgst {
						return nil, nil, errdefs.NotFound(errors.New("ambiguous reference"))
					}
				} else {
					dgst = limg.Target.Digest
				}
			}

			// Return immediately if target digest matches already included
			if img == nil || len(imgs) > 1 {
				return img, imgs, nil
			}
		}
	} else {
		named, ok := parsed.(reference.Named)
		if !ok {
			return nil, nil, errdefs.InvalidParameter(errors.New("invalid name reference"))
		}

		digested, ok := parsed.(reference.Digested)
		if ok {
			dgst = digested.Digest()
		}

		name := reference.TagNameOnly(named).String()

		cimg, err := i.images.Get(ctx, name)
		if err != nil {
			if !cerrdefs.IsNotFound(err) {
				return nil, nil, convertError(err)
			}
			// If digest is given, continue looking up for matching targets.
			// There will be no exact match found but the caller may attempt
			// to match across images with the matching target.
			if dgst == "" {
				return nil, nil, images.ErrImageDoesNotExist{Ref: parsed}
			}
		} else {
			img = &cimg
			if dgst != "" && img.Target.Digest != dgst {
				// Ambiguous image reference, use reference name
				log.G(ctx).WithField("image", name).WithField("target", cimg.Target.Digest).Warn("digest reference points to image with a different digest")
			}
			dgst = img.Target.Digest
		}
	}

	// Lookup up all associated images and check for consistency with first reference
	// Ideally operations dependent on multiple values will rely on the garbage collector,
	// this logic will just check for consistency and throw an error
	imgs, err := i.images.List(ctx, "target.digest=="+dgst.String())
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to lookup digest")
	}
	if len(imgs) == 0 {
		if img == nil {
			return nil, nil, images.ErrImageDoesNotExist{Ref: parsed}
		}
		err = errInconsistentData
	} else if img != nil {
		// Check to ensure the original img is in the list still
		err = errInconsistentData
		for _, rimg := range imgs {
			if rimg.Name == img.Name {
				err = nil
				break
			}
		}
	}
	if errors.Is(err, errInconsistentData) {
		if retries, ok := ctx.Value(errInconsistentData).(int); !ok || retries < 3 {
			log.G(ctx).WithFields(log.Fields{"retry": retries, "ref": refOrID}).Info("image changed during lookup, retrying")
			return i.resolveAllReferences(context.WithValue(ctx, errInconsistentData, retries+1), refOrID)
		}
		return nil, nil, err
	}

	return img, imgs, nil
}
