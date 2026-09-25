package contentutil

import (
	"context"

	"github.com/containerd/containerd/v2/core/content"
	ocispecs "github.com/opencontainers/image-spec/specs-go/v1"
)

func NewStoreWithProvider(cs content.Store, p content.Provider) content.Store {
	return &storeWithProvider{Store: cs, p: p}
}

type storeWithProvider struct {
	content.Store
	p content.Provider
}

func (cs *storeWithProvider) ReaderAt(ctx context.Context, desc ocispecs.Descriptor) (content.ReaderAt, error) {
	if ra, err := cs.p.ReaderAt(ctx, desc); err == nil {
		return ra, nil
	}
	return cs.Store.ReaderAt(ctx, desc)
}
