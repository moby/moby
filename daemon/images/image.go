package images // import "github.com/docker/docker/daemon/images"

import (
	"fmt"

	"github.com/containerd/containerd/platforms"
	"github.com/pkg/errors"

	"github.com/docker/distribution/reference"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/image"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
)

// ErrImageDoesNotExist is error returned when no image can be found for a reference.
type ErrImageDoesNotExist struct {
	ref reference.Reference
}

func (e ErrImageDoesNotExist) Error() string {
	ref := e.ref
	if named, ok := ref.(reference.Named); ok {
		ref = reference.TagNameOnly(named)
	}
	return fmt.Sprintf("No such image: %s", reference.FamiliarString(ref))
}

// NotFound implements the NotFound interface
func (e ErrImageDoesNotExist) NotFound() {}

// GetImage returns an image corresponding to the image referred to by refOrID.
func (i *ImageService) GetImage(refOrID string, platform *specs.Platform) (retImg *image.Image, retErr error) {
	defer func() {
		if retErr != nil || retImg == nil || platform == nil {
			return
		}

		imgPlat := specs.Platform{
			OS:           retImg.OS,
			Architecture: retImg.Architecture,
			Variant:      retImg.Variant,
		}
		p := *platform
		// Note that `platforms.Only` will fuzzy match this for us
		// For example: an armv6 image will run just fine an an armv7 CPU, without emulation or anything.
		if !platforms.Only(p).Match(imgPlat) {
			// This allows us to tell clients that we don't have the image they asked for
			// Where this gets hairy is the image store does not currently support multi-arch images, e.g.:
			//   An image `foo` may have a multi-arch manifest, but the image store only fetches the image for a specific platform
			//   The image store does not store the manifest list and image tags are assigned to architecture specific images.
			//   So we can have a `foo` image that is amd64 but the user requested armv7. If the user looks at the list of images.
			//   This may be confusing.
			//   The alternative to this is to return a errdefs.Conflict error with a helpful message, but clients will not be
			//   able to automatically tell what causes the conflict.
			retErr = errdefs.NotFound(errors.Errorf("image with reference %s was found but does not match the specified platform: wanted %s, actual: %s", refOrID, platforms.Format(p), platforms.Format(imgPlat)))
			return
		}
	}()
	ref, err := reference.ParseAnyReference(refOrID)
	if err != nil {
		return nil, errdefs.InvalidParameter(err)
	}
	namedRef, ok := ref.(reference.Named)
	if !ok {
		digested, ok := ref.(reference.Digested)
		if !ok {
			return nil, ErrImageDoesNotExist{ref}
		}
		id := image.IDFromDigest(digested.Digest())
		if img, err := i.imageStore.Get(id); err == nil {
			return img, nil
		}
		return nil, ErrImageDoesNotExist{ref}
	}

	if digest, err := i.referenceStore.Get(namedRef); err == nil {
		// Search the image stores to get the operating system, defaulting to host OS.
		id := image.IDFromDigest(digest)
		if img, err := i.imageStore.Get(id); err == nil {
			return img, nil
		}
	}

	// Search based on ID
	if id, err := i.imageStore.Search(refOrID); err == nil {
		img, err := i.imageStore.Get(id)
		if err != nil {
			return nil, ErrImageDoesNotExist{ref}
		}
		return img, nil
	}

	return nil, ErrImageDoesNotExist{ref}
}
