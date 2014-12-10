package graph

import (
	"errors"
	"fmt"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/docker/image"
	"github.com/docker/docker/pkg/archive"
)

// SquashLayers consolidates all layers in between the given imageID and
// ancestorID into a single layer. Returns the ID of the newly created image.
func (graph *Graph) SquashLayers(imageID, ancestorID string) (squashedID string, err error) {
	log.Debugf("squashing image ID %q up to %q", imageID, ancestorID)

	if imageID == ancestorID {
		// No need to squash! They are the same image already!
		return imageID, nil
	}

	var (
		img           *image.Image
		squashedImage *image.Image
		squashDiff    archive.Archive
	)

	if img, err = graph.Get(imageID); err != nil {
		return "", err
	}

	if img.Parent == ancestorID {
		// No need to squash! It's already only a single layer difference!
		return imageID, nil
	}

	if ancestorID != "" {
		// Ensure that the ancestorID is an actual ancestor
		// by walking the image history to find it.
		foundAncestor := errors.New("")
		err = img.WalkHistory(func(ancestorImg *image.Image) error {
			if ancestorImg.ID == ancestorID {
				return foundAncestor // Exit walk early.
			}
			return nil
		})
		if err == nil {
			return "", fmt.Errorf("unable to squash: %q is not an ancestor of %q", ancestorID, imageID)
		}
		if err != foundAncestor {
			return "", err
		}
	}

	// The special sauce! Actually getting an diff from the ancestor Image rootfs.
	if squashDiff, err = graph.Driver().DiffAncestor(imageID, ancestorID); err != nil {
		return "", err
	}
	defer squashDiff.Close()

	// Create it like any other image is created!
	if squashedImage, err = graph.Create(squashDiff, "", ancestorID, img.Comment, img.Author, nil, img.Config); err != nil {
		return "", err
	}

	return squashedImage.ID, nil
}
