package containerd

import (
	"context"

	"github.com/containerd/containerd/content"
	cerrdefs "github.com/containerd/containerd/errdefs"
	containerdimages "github.com/containerd/containerd/images"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// presentChildrenHandler is a handler wrapper which traverses all children
// descriptors that are present in the store and calls specified handler.
func presentChildrenHandler(store content.Store, h containerdimages.HandlerFunc) containerdimages.HandlerFunc {
	return func(ctx context.Context, desc ocispec.Descriptor) ([]ocispec.Descriptor, error) {
		var children []ocispec.Descriptor

		_, err := store.Info(ctx, desc.Digest)
		if err != nil {
			if cerrdefs.IsNotFound(err) {
				return nil, nil
			}
			return nil, err
		} else {
			c, err := h(ctx, desc)
			if err != nil {
				return children, err
			}

			children = append(children, c...)
		}

		c, err := containerdimages.Children(ctx, store, desc)
		if err != nil {
			return children, err
		}
		children = append(children, c...)

		return children, nil
	}
}
