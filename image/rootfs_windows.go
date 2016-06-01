// +build windows

package image

import (
	"crypto/sha512"
	"fmt"

	"github.com/docker/distribution/digest"
	"github.com/docker/docker/layer"
)

// TypeLayersWithBase is used for RootFS.Type for Windows filesystems that have layers and a centrally-stored base layer.
const TypeLayersWithBase = "layers+base"

// RootFS describes images root filesystem
// This is currently a placeholder that only supports layers. In the future
// this can be made into an interface that supports different implementations.
type RootFS struct {
	Type      string         `json:"type"`
	DiffIDs   []layer.DiffID `json:"diff_ids,omitempty"`
	BaseLayer string         `json:"base_layer,omitempty"`
}

// BaseLayerID returns the 64 byte hex ID for the baselayer name.
func (r *RootFS) BaseLayerID() string {
	if r.Type != TypeLayersWithBase {
		panic("tried to get base layer ID without a base layer")
	}
	baseID := sha512.Sum384([]byte(r.BaseLayer))
	return fmt.Sprintf("%x", baseID[:32])
}

// ChainID returns the ChainID for the top layer in RootFS.
func (r *RootFS) ChainID() layer.ChainID {
	ids := r.DiffIDs
	if r.Type == TypeLayersWithBase {
		// Add an extra ID for the base.
		baseDiffID := layer.DiffID(digest.FromBytes([]byte(r.BaseLayerID())))
		ids = append([]layer.DiffID{baseDiffID}, ids...)
	}
	return layer.CreateChainID(ids)
}

// NewRootFSWithBaseLayer returns a RootFS struct with a base layer
func NewRootFSWithBaseLayer(baseLayer string) *RootFS {
	return &RootFS{Type: TypeLayersWithBase, BaseLayer: baseLayer}
}
