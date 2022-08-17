package containerd

import imagetype "github.com/docker/docker/api/types/image"

// ImageHistory returns a slice of ImageHistory structures for the specified
// image name by walking the image lineage.
func (i *ImageService) ImageHistory(name string) ([]*imagetype.HistoryResponseItem, error) {
	panic("not implemented")
}
