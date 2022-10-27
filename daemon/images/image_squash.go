package images // import "github.com/docker/docker/daemon/images"

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/docker/docker/image"
	"github.com/docker/docker/layer"
	"github.com/pkg/errors"
)

// SquashImage creates a new image with the diff of the specified image and the specified parent.
// This new image contains only the layers from it's parent + 1 extra layer which contains the diff of all the layers in between.
// The existing image(s) is not destroyed.
// If no parent is specified, a new image with the diff of all the specified image's layers merged into a new layer that has no parents.
func (i *ImageService) SquashImage(id, parent string) (string, error) {
	var (
		img *image.Image
		err error
	)
	if img, err = i.imageStore.Get(image.ID(id)); err != nil {
		return "", err
	}

	var parentImg *image.Image
	var parentChainID layer.ChainID
	if len(parent) != 0 {
		parentImg, err = i.imageStore.Get(image.ID(parent))
		if err != nil {
			return "", errors.Wrap(err, "error getting specified parent layer")
		}
		parentChainID = parentImg.RootFS.ChainID()
	} else {
		rootFS := image.NewRootFS()
		parentImg = &image.Image{RootFS: rootFS}
	}
	l, err := i.layerStore.Get(img.RootFS.ChainID())
	if err != nil {
		return "", errors.Wrap(err, "error getting image layer")
	}
	defer i.layerStore.Release(l)

	ts, err := l.TarStreamFrom(parentChainID)
	if err != nil {
		return "", errors.Wrapf(err, "error getting tar stream to parent")
	}
	defer ts.Close()

	newL, err := i.layerStore.Register(ts, parentChainID)
	if err != nil {
		return "", errors.Wrap(err, "error registering layer")
	}
	defer i.layerStore.Release(newL)

	newImage := *img
	newImage.RootFS = nil

	rootFS := *parentImg.RootFS
	rootFS.DiffIDs = append(rootFS.DiffIDs, newL.DiffID())
	newImage.RootFS = &rootFS

	for i, hi := range newImage.History {
		if i >= len(parentImg.History) {
			hi.EmptyLayer = true
		}
		newImage.History[i] = hi
	}

	now := time.Now()
	var historyComment string
	if len(parent) > 0 {
		historyComment = fmt.Sprintf("merge %s to %s", id, parent)
	} else {
		historyComment = fmt.Sprintf("create new from %s", id)
	}

	newImage.History = append(newImage.History, image.History{
		Created: now,
		Comment: historyComment,
	})
	newImage.Created = now

	b, err := json.Marshal(&newImage)
	if err != nil {
		return "", errors.Wrap(err, "error marshalling image config")
	}

	newImgID, err := i.imageStore.Create(b)
	if err != nil {
		return "", errors.Wrap(err, "error creating new image after squash")
	}
	return string(newImgID), nil
}
