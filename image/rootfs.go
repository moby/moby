package image

import "github.com/docker/docker/layer"

// TypeLayers is used for RootFS.Type for filesystems organized into layers.
const TypeLayers = "layers"

// NewRootFS returns empty RootFS struct
func NewRootFS() *RootFS {
	return &RootFS{Type: TypeLayers}
}

// Append appends a new diffID to rootfs
func (r *RootFS) Append(id layer.DiffID) {
	r.DiffIDs = append(r.DiffIDs, id)
}
