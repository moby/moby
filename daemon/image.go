package daemon

import (
	"fmt"

	"github.com/docker/distribution/reference"
	"github.com/docker/docker/builder"
	"github.com/docker/docker/image"
	"github.com/docker/docker/pkg/stringid"
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

// GetImageID returns an image ID corresponding to the image referred to by
// refOrID.
func (daemon *Daemon) GetImageID(refOrID string) (image.ID, error) {
	ref, err := reference.ParseAnyReference(refOrID)
	if err != nil {
		return "", err
	}
	namedRef, ok := ref.(reference.Named)
	if !ok {
		digested, ok := ref.(reference.Digested)
		if !ok {
			return "", ErrImageDoesNotExist{ref}
		}
		id := image.IDFromDigest(digested.Digest())
		if _, err := daemon.imageStore.Get(id); err != nil {
			return "", ErrImageDoesNotExist{ref}
		}
		return id, nil
	}

	if id, err := daemon.referenceStore.Get(namedRef); err == nil {
		return image.IDFromDigest(id), nil
	}

	// deprecated: repo:shortid https://github.com/docker/docker/pull/799
	if tagged, ok := namedRef.(reference.Tagged); ok {
		if tag := tagged.Tag(); stringid.IsShortID(stringid.TruncateID(tag)) {
			if id, err := daemon.imageStore.Search(tag); err == nil {
				for _, storeRef := range daemon.referenceStore.References(id.Digest()) {
					if storeRef.Name() == namedRef.Name() {
						return id, nil
					}
				}
			}
		}
	}

	// Search based on ID
	if id, err := daemon.imageStore.Search(refOrID); err == nil {
		return id, nil
	}

	return "", ErrImageDoesNotExist{ref}
}

// GetImage returns an image corresponding to the image referred to by refOrID.
func (daemon *Daemon) GetImage(refOrID string) (*image.Image, error) {
	imgID, err := daemon.GetImageID(refOrID)
	if err != nil {
		return nil, err
	}
	return daemon.imageStore.Get(imgID)
}

// GetImageOnBuild looks up a Docker image referenced by `name`.
func (daemon *Daemon) GetImageOnBuild(name string) (builder.Image, error) {
	img, err := daemon.GetImage(name)
	if err != nil {
		return nil, err
	}
	return img, nil
}
