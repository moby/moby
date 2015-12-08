// +build windows

package image

import (
	"crypto/sha512"
	"fmt"

	"github.com/docker/distribution/digest"
	"github.com/docker/docker/layer"
)

// RootFS describes images root filesystem
// This is currently a placeholder that only supports layers. In the future
// this can be made into a interface that supports different implementaions.
type RootFS struct {
	Type      string         `json:"type"`
	DiffIDs   []layer.DiffID `json:"diff_ids,omitempty"`
	BaseLayer string         `json:"base_layer,omitempty"`
}

// BaseLayerID returns the 64 byte hex ID for the baselayer name.
func (r *RootFS) BaseLayerID() string {
	baseID := sha512.Sum384([]byte(r.BaseLayer))
	return fmt.Sprintf("%x", baseID[:32])
}

// ChainID returns the ChainID for the top layer in RootFS.
func (r *RootFS) ChainID() layer.ChainID {
	baseDiffID, _ := digest.FromBytes([]byte(r.BaseLayerID())) // can never error
	return layer.CreateChainID(append([]layer.DiffID{layer.DiffID(baseDiffID)}, r.DiffIDs...))
}

// NewRootFS returns empty RootFS struct
func NewRootFS() *RootFS {
	return &RootFS{Type: "layers+base"}
}
