package daemon

import (
	"fmt"
	"runtime"

	"github.com/docker/distribution/reference"
	"github.com/docker/docker/image"
	"github.com/docker/docker/pkg/stringid"
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

// GetImageIDAndPlatform returns an image ID and platform corresponding to the image referred to by
// refOrID.
func (daemon *Daemon) GetImageIDAndPlatform(refOrID string) (image.ID, string, error) {
	ref, err := reference.ParseAnyReference(refOrID)
	if err != nil {
		return "", "", validationError{err}
	}
	namedRef, ok := ref.(reference.Named)
	if !ok {
		digested, ok := ref.(reference.Digested)
		if !ok {
			return "", "", errImageDoesNotExist{ref}
		}
		id := image.IDFromDigest(digested.Digest())
		for platform := range daemon.stores {
			if _, err = daemon.stores[platform].imageStore.Get(id); err == nil {
				return id, platform, nil
			}
		}
		return "", "", errImageDoesNotExist{ref}
	}

	if digest, err := daemon.referenceStore.Get(namedRef); err == nil {
		// Search the image stores to get the platform, defaulting to host OS.
		imagePlatform := runtime.GOOS
		id := image.IDFromDigest(digest)
		for platform := range daemon.stores {
			if img, err := daemon.stores[platform].imageStore.Get(id); err == nil {
				imagePlatform = img.Platform()
				break
			}
		}
		return id, imagePlatform, nil
	}

	// deprecated: repo:shortid https://github.com/docker/docker/pull/799
	if tagged, ok := namedRef.(reference.Tagged); ok {
		if tag := tagged.Tag(); stringid.IsShortID(stringid.TruncateID(tag)) {
			for platform := range daemon.stores {
				if id, err := daemon.stores[platform].imageStore.Search(tag); err == nil {
					for _, storeRef := range daemon.referenceStore.References(id.Digest()) {
						if storeRef.Name() == namedRef.Name() {
							return id, platform, nil
						}
					}
				}
			}
		}
	}

	// Search based on ID
	for platform := range daemon.stores {
		if id, err := daemon.stores[platform].imageStore.Search(refOrID); err == nil {
			return id, platform, nil
		}
	}

	return "", "", errImageDoesNotExist{ref}
}

// GetImage returns an image corresponding to the image referred to by refOrID.
func (daemon *Daemon) GetImage(refOrID string) (*image.Image, error) {
	imgID, platform, err := daemon.GetImageIDAndPlatform(refOrID)
	if err != nil {
		return nil, err
	}
	return daemon.stores[platform].imageStore.Get(imgID)
}
