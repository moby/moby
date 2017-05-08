package image

import "github.com/docker/docker/layer"

// TypeLayers is used for RootFS.Type for filesystems organized into layers.
const TypeLayers = "layers"

// RootFS describes images root filesystem
// This is currently a placeholder that only supports layers. In the future
// this can be made into an interface that supports different implementations.
type RootFS struct {
	Type    string         `json:"type"`
	DiffIDs []layer.DiffID `json:"diff_ids,omitempty"`
}

// NewRootFS returns empty RootFS struct
func NewRootFS() *RootFS {
	return &RootFS{Type: TypeLayers}
}

// Append appends a new diffID to rootfs
func (r *RootFS) Append(id layer.DiffID) {
	r.DiffIDs = append(r.DiffIDs, id)
}

// ChainID returns the ChainID for the top layer in RootFS.
func (r *RootFS) ChainID() layer.ChainID {
	return layer.CreateChainID(r.DiffIDs)
}
