package containerd

import (
	"context"

	"github.com/containerd/containerd/content"
	c8dimages "github.com/containerd/containerd/images"
	cerrdefs "github.com/containerd/errdefs"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// walkPresentChildren is a simple wrapper for c8dimages.Walk with presentChildrenHandler.
// This is only a convenient helper to reduce boilerplate.
func (i *ImageService) walkPresentChildren(ctx context.Context, target ocispec.Descriptor, f func(context.Context, ocispec.Descriptor) error) error {
	return c8dimages.Walk(ctx, presentChildrenHandler(i.content, c8dimages.HandlerFunc(
		func(ctx context.Context, desc ocispec.Descriptor) ([]ocispec.Descriptor, error) {
			return nil, f(ctx, desc)
		})), target)
}

// presentChildrenHandler is a handler wrapper which traverses all children
// descriptors that are present in the store and calls specified handler.
func presentChildrenHandler(store content.Store, h c8dimages.HandlerFunc) c8dimages.HandlerFunc {
	return func(ctx context.Context, desc ocispec.Descriptor) ([]ocispec.Descriptor, error) {
		_, err := store.Info(ctx, desc.Digest)
		if err != nil {
			if cerrdefs.IsNotFound(err) {
				return nil, nil
			}
			return nil, err
		}

		children, err := h(ctx, desc)
		if err != nil {
			return nil, err
		}

		c, err := c8dimages.Children(ctx, store, desc)
		if err != nil {
			return nil, err
		}
		children = append(children, c...)

		return children, nil
	}
}
