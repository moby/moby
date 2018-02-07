package images // import "github.com/docker/docker/daemon/images"

import (
	"fmt"

	"github.com/docker/distribution/reference"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/image"
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

// GetImageIDAndOS returns an image ID and operating system corresponding to the image referred to by
// refOrID.
// called from list.go foldFilter()
func (i ImageService) GetImageIDAndOS(refOrID string) (image.ID, string, error) {
	ref, err := reference.ParseAnyReference(refOrID)
	if err != nil {
		return "", "", errdefs.InvalidParameter(err)
	}
	namedRef, ok := ref.(reference.Named)
	if !ok {
		digested, ok := ref.(reference.Digested)
		if !ok {
			return "", "", ErrImageDoesNotExist{ref}
		}
		id := image.IDFromDigest(digested.Digest())
		if img, err := i.imageStore.Get(id); err == nil {
			return id, img.OperatingSystem(), nil
		}
		return "", "", ErrImageDoesNotExist{ref}
	}

	if digest, err := i.referenceStore.Get(namedRef); err == nil {
		// Search the image stores to get the operating system, defaulting to host OS.
		id := image.IDFromDigest(digest)
		if img, err := i.imageStore.Get(id); err == nil {
			return id, img.OperatingSystem(), nil
		}
	}

	// Search based on ID
	if id, err := i.imageStore.Search(refOrID); err == nil {
		img, err := i.imageStore.Get(id)
		if err != nil {
			return "", "", ErrImageDoesNotExist{ref}
		}
		return id, img.OperatingSystem(), nil
	}

	return "", "", ErrImageDoesNotExist{ref}
}

// GetImage returns an image corresponding to the image referred to by refOrID.
func (i *ImageService) GetImage(refOrID string) (*image.Image, error) {
	imgID, _, err := i.GetImageIDAndOS(refOrID)
	if err != nil {
		return nil, err
	}
	return i.imageStore.Get(imgID)
}
