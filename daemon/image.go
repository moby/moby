package daemon

import (
	"fmt"

	"github.com/docker/docker/builder"
	"github.com/docker/docker/image"
	"github.com/docker/docker/reference"
	"github.com/docker/docker/runconfig"
	containertypes "github.com/docker/engine-api/types/container"
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
		if _, err := daemon.imageStore.Get(image.ID(id)); err != nil {
			return "", ErrImageDoesNotExist{refOrID}
		}
		return image.ID(id), nil
	}

	if id, err := daemon.referenceStore.Get(ref); err == nil {
		return id, nil
	}
	if tagged, ok := ref.(reference.NamedTagged); ok {
		if id, err := daemon.imageStore.Search(tagged.Tag()); err == nil {
			for _, namedRef := range daemon.referenceStore.References(id) {
				if namedRef.Name() == ref.Name() {
					return id, nil
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

// GetCachedImage returns the most recent created image that is a child
// of the image with imgID, that had the same config when it was
// created. nil is returned if a child cannot be found. An error is
// returned if the parent image cannot be found.
func (daemon *Daemon) GetCachedImage(imgID image.ID, config *containertypes.Config) (*image.Image, error) {
	// Loop on the children of the given image and check the config
	getMatch := func(siblings []image.ID) (*image.Image, error) {
		var match *image.Image
		for _, id := range siblings {
			img, err := daemon.imageStore.Get(id)
			if err != nil {
				return nil, fmt.Errorf("unable to find image %q", id)
			}

			if runconfig.Compare(&img.ContainerConfig, config) {
				// check for the most up to date match
				if match == nil || match.Created.Before(img.Created) {
					match = img
				}
			}
		}
		return match, nil
	}

	// In this case, this is `FROM scratch`, which isn't an actual image.
	if imgID == "" {
		images := daemon.imageStore.Map()
		var siblings []image.ID
		for id, img := range images {
			if img.Parent == imgID {
				siblings = append(siblings, id)
			}
		}
		return getMatch(siblings)
	}

	// find match from child images
	siblings := daemon.imageStore.Children(imgID)
	return getMatch(siblings)
}

// GetCachedImageOnBuild returns a reference to a cached image whose parent equals `parent`
// and runconfig equals `cfg`. A cache miss is expected to return an empty ID and a nil error.
func (daemon *Daemon) GetCachedImageOnBuild(imgID string, cfg *containertypes.Config) (string, error) {
	cache, err := daemon.GetCachedImage(image.ID(imgID), cfg)
	if cache == nil || err != nil {
		return "", err
	}
	return cache.ID().String(), nil
}
