package daemon

import (
	"fmt"

	"github.com/docker/docker/builder"
	"github.com/docker/docker/image"
	"github.com/docker/docker/pkg/stringid"
	"github.com/docker/docker/reference"
)

// ErrImageDoesNotExist is error returned when no image can be found for a reference.
type ErrImageDoesNotExist struct {
	RefOrID string
}

func (e ErrImageDoesNotExist) Error() string {
	return fmt.Sprintf("no such id: %s", e.RefOrID)
}

// GetImageID returns an image ID corresponding to the image referred to by
// refOrID.
func (daemon *Daemon) GetImageID(refOrID string) (image.ID, error) {
	id, ref, err := reference.ParseIDOrReference(refOrID)
	if err != nil {
		return "", err
	}
	if id != "" {
		if _, err := daemon.imageStore.Get(image.IDFromDigest(id)); err != nil {
			return "", ErrImageDoesNotExist{refOrID}
		}
		return image.IDFromDigest(id), nil
	}

	if id, err := daemon.referenceStore.Get(ref); err == nil {
		return image.IDFromDigest(id), nil
	}

	// deprecated: repo:shortid https://github.com/docker/docker/pull/799
	if tagged, ok := ref.(reference.NamedTagged); ok {
		if tag := tagged.Tag(); stringid.IsShortID(stringid.TruncateID(tag)) {
			if id, err := daemon.imageStore.Search(tag); err == nil {
				for _, namedRef := range daemon.referenceStore.References(id.Digest()) {
					if namedRef.Name() == ref.Name() {
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

	return "", ErrImageDoesNotExist{refOrID}
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
