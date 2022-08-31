package containerd

import "errors"

// SquashImage creates a new image with the diff of the specified image and
// the specified parent. This new image contains only the layers from its
// parent + 1 extra layer which contains the diff of all the layers in between.
// The existing image(s) is not destroyed. If no parent is specified, a new
// image with the diff of all the specified image's layers merged into a new
// layer that has no parents.
func (i *ImageService) SquashImage(id, parent string) (string, error) {
	return "", errors.New("not implemented")
}
