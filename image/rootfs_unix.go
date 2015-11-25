// +build !windows

package image

import "github.com/docker/docker/layer"

// RootFS describes images root filesystem
// This is currently a placeholder that only supports layers. In the future
// this can be made into a interface that supports different implementaions.
type RootFS struct {
	Type    string         `json:"type"`
	DiffIDs []layer.DiffID `json:"diff_ids,omitempty"`
}

// ChainID returns the ChainID for the top layer in RootFS.
func (r *RootFS) ChainID() layer.ChainID {
	return layer.CreateChainID(r.DiffIDs)
}

// NewRootFS returns empty RootFS struct
func NewRootFS() *RootFS {
	return &RootFS{Type: "layers"}
}
