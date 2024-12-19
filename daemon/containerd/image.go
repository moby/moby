package containerd

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	c8dimages "github.com/containerd/containerd/images"
	cerrdefs "github.com/containerd/errdefs"
	"github.com/containerd/log"
	"github.com/containerd/platforms"
	"github.com/distribution/reference"
	"github.com/docker/docker/api/types/backend"
	"github.com/docker/docker/daemon/images"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/image"
	imagespec "github.com/moby/docker-image-spec/specs-go/v1"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

var errInconsistentData error = errors.New("consistency error: data changed during operation, retry")

type errPlatformNotFound struct {
	wanted   ocispec.Platform
	imageRef string
}

func (e *errPlatformNotFound) NotFound() {}
func (e *errPlatformNotFound) Error() string {
	msg := "image with reference " + e.imageRef + " was found but does not provide the specified platform"
	if e.wanted.OS != "" {
		msg += " (" + platforms.FormatAll(e.wanted) + ")"
	}
	return msg
}

// GetImage returns an image corresponding to the image referred to by refOrID.
func (i *ImageService) GetImage(ctx context.Context, refOrID string, options backend.GetImageOpts) (*image.Image, error) {
	img, err := i.resolveImage(ctx, refOrID)
	if err != nil {
		return nil, err
	}

	pm := i.matchRequestedOrDefault(platforms.OnlyStrict, options.Platform)

	im, err := i.getBestPresentImageManifest(ctx, img, pm)
	if err != nil {
		return nil, err
	}

	var ociImage imagespec.DockerOCIImage
	err = im.ReadConfig(ctx, &ociImage)
	if err != nil {
		return nil, err
	}
	imgV1 := dockerOciImageToDockerImagePartial(image.ID(img.Target.Digest), ociImage)

	parent, err := i.getImageLabelByDigest(ctx, img.Target.Digest, imageLabelClassicBuilderParent)
	if err != nil {
		log.G(ctx).WithError(err).Warn("failed to determine Parent property")
	} else {
		imgV1.Parent = image.ID(parent)
	}

	target := im.Target()
	imgV1.Details = &image.Details{
		ManifestDescriptor: &target,
	}

	return imgV1, nil
}

// getBestPresentImageManifest returns a platform-specific image manifest that best matches the provided platform matcher.
// Only locally available platform images are considered.
// If no image manifest matches the platform, an error is returned.
func (i *ImageService) getBestPresentImageManifest(ctx context.Context, img c8dimages.Image, pm platforms.MatchComparer) (*ImageManifest, error) {
	var best *ImageManifest
	var bestPlatform ocispec.Platform

	err := i.walkImageManifests(ctx, img, func(im *ImageManifest) error {
		imPlatform, err := im.ImagePlatform(ctx)
		if err != nil {
			return err
		}

		if pm.Match(imPlatform) {
			if best == nil || pm.Less(imPlatform, bestPlatform) {
				best = im
				bestPlatform = imPlatform
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	if best == nil {
		err := &errPlatformNotFound{imageRef: imageFamiliarName(img)}
		if p, ok := pm.(platformMatcherWithRequestedPlatform); ok && p.Requested != nil {
			err.wanted = *p.Requested
		}
		return nil, err
	}

	return best, nil
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

// ResolveImage looks up an image by reference or identifier in the image store.
func (i *ImageService) ResolveImage(ctx context.Context, refOrID string) (c8dimages.Image, error) {
	return i.resolveImage(ctx, refOrID)
}

func (i *ImageService) resolveImage(ctx context.Context, refOrID string) (c8dimages.Image, error) {
	parsed, err := reference.ParseAnyReference(refOrID)
	if err != nil {
		return c8dimages.Image{}, errdefs.InvalidParameter(err)
	}

	digested, ok := parsed.(reference.Digested)
	if ok {
		imgs, err := i.images.List(ctx, "target.digest=="+digested.Digest().String())
		if err != nil {
			return c8dimages.Image{}, errors.Wrap(err, "failed to lookup digest")
		}
		if len(imgs) == 0 {
			return c8dimages.Image{}, images.ErrImageDoesNotExist{Ref: parsed}
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
			return c8dimages.Image{}, images.ErrImageDoesNotExist{Ref: parsed}
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
			return c8dimages.Image{}, err
		}
	}

	// If the identifier could be a short ID, attempt to match.
	if idWithoutAlgo := checkTruncatedID(refOrID); idWithoutAlgo != "" { // Valid ID.
		filters := []string{
			fmt.Sprintf("name==%q", ref), // Or it could just look like one.
			"target.digest~=" + strconv.Quote(fmt.Sprintf(`^sha256:%s[0-9a-fA-F]{%d}$`, regexp.QuoteMeta(idWithoutAlgo), 64-len(idWithoutAlgo))),
		}
		imgs, err := i.images.List(ctx, filters...)
		if err != nil {
			return c8dimages.Image{}, err
		}

		if len(imgs) == 0 {
			return c8dimages.Image{}, images.ErrImageDoesNotExist{Ref: parsed}
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
				return c8dimages.Image{}, errdefs.NotFound(errors.New("ambiguous reference"))
			}
		}

		return imgs[0], nil
	}

	return c8dimages.Image{}, images.ErrImageDoesNotExist{Ref: parsed}
}

// getAllImagesWithRepository returns a slice of images which name is a reference
// pointing to the same repository as the given reference.
func (i *ImageService) getAllImagesWithRepository(ctx context.Context, ref reference.Named) ([]c8dimages.Image, error) {
	nameFilter := "^" + regexp.QuoteMeta(ref.Name()) + ":" + reference.TagRegexp.String() + "$"
	return i.images.List(ctx, "name~="+strconv.Quote(nameFilter))
}

func imageFamiliarName(img c8dimages.Image) string {
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
	imgs, err := i.images.List(ctx, "target.digest=="+target.String()+",labels."+labelKey)
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
func (i *ImageService) resolveAllReferences(ctx context.Context, refOrID string) (*c8dimages.Image, []c8dimages.Image, error) {
	parsed, err := reference.ParseAnyReference(refOrID)
	if err != nil {
		return nil, nil, errdefs.InvalidParameter(err)
	}
	var dgst digest.Digest
	var img *c8dimages.Image

	if idWithoutAlgo := checkTruncatedID(refOrID); idWithoutAlgo != "" { // Valid ID.
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

// checkTruncatedID checks id for validity. If id is invalid, an empty string
// is returned; otherwise, the ID without the optional "sha256:" prefix is
// returned. The validity check is equivalent to
// regexp.MustCompile(`^(sha256:)?([a-f0-9]{4,64})$`).MatchString(id).
func checkTruncatedID(id string) string {
	id = strings.TrimPrefix(id, "sha256:")
	if l := len(id); l < 4 || l > 64 {
		return ""
	}
	for _, c := range id {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return ""
		}
	}
	return id
}
