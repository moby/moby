package daemon // import "github.com/docker/docker/daemon"

import (
	"fmt"

	"github.com/docker/distribution/reference"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/image"
)

// errImageDoesNotExist is error returned when no image can be found for a reference.
type errImageDoesNotExist struct {
	ref reference.Reference
}

func (e errImageDoesNotExist) Error() string {
	ref := e.ref
	if named, ok := ref.(reference.Named); ok {
		ref = reference.TagNameOnly(named)
	}
	return fmt.Sprintf("No such image: %s", reference.FamiliarString(ref))
}

func (e errImageDoesNotExist) NotFound() {}

// GetImageIDAndOS returns an image ID and operating system corresponding to the image referred to by
// refOrID.
func (daemon *Daemon) GetImageIDAndOS(refOrID string) (image.ID, string, error) {
	ref, err := reference.ParseAnyReference(refOrID)
	if err != nil {
		return "", "", errdefs.InvalidParameter(err)
	}
	namedRef, ok := ref.(reference.Named)
	if !ok {
		digested, ok := ref.(reference.Digested)
		if !ok {
			return "", "", errImageDoesNotExist{ref}
		}
		id := image.IDFromDigest(digested.Digest())
		if img, err := daemon.imageStore.Get(id); err == nil {
			return id, img.OperatingSystem(), nil
		}
		return "", "", errImageDoesNotExist{ref}
	}

	if digest, err := daemon.referenceStore.Get(namedRef); err == nil {
		// Search the image stores to get the operating system, defaulting to host OS.
		id := image.IDFromDigest(digest)
		if img, err := daemon.imageStore.Get(id); err == nil {
			return id, img.OperatingSystem(), nil
		}
	}

	// Search based on ID
	if id, err := daemon.imageStore.Search(refOrID); err == nil {
		img, err := daemon.imageStore.Get(id)
		if err != nil {
			return "", "", errImageDoesNotExist{ref}
		}
		return id, img.OperatingSystem(), nil
	}

	return "", "", errImageDoesNotExist{ref}
}

// GetImage returns an image corresponding to the image referred to by refOrID.
func (daemon *Daemon) GetImage(refOrID string) (*image.Image, error) {
	imgID, _, err := daemon.GetImageIDAndOS(refOrID)
	if err != nil {
		return nil, err
	}
	return daemon.imageStore.Get(imgID)
}
