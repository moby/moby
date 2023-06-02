package containerd

import (
	"context"

	"github.com/containerd/containerd/content"
	cerrdefs "github.com/containerd/containerd/errdefs"
	containerdimages "github.com/containerd/containerd/images"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// walkPresentChildren is a simple wrapper for containerdimages.Walk with presentChildrenHandler.
// This is only a convenient helper to reduce boilerplate.
func (i *ImageService) walkPresentChildren(ctx context.Context, target ocispec.Descriptor, f func(context.Context, ocispec.Descriptor) error) error {
	store := i.client.ContentStore()
	return containerdimages.Walk(ctx, presentChildrenHandler(store, containerdimages.HandlerFunc(
		func(ctx context.Context, desc ocispec.Descriptor) ([]ocispec.Descriptor, error) {
			return nil, f(ctx, desc)
		})), target)
}

// presentChildrenHandler is a handler wrapper which traverses all children
// descriptors that are present in the store and calls specified handler.
func presentChildrenHandler(store content.Store, h containerdimages.HandlerFunc) containerdimages.HandlerFunc {
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

		c, err := containerdimages.Children(ctx, store, desc)
		if err != nil {
			return nil, err
		}
		children = append(children, c...)

		return children, nil
	}
}
