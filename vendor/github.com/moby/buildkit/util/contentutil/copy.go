package contentutil

import (
	"context"
	"io"

	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/remotes"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

func Copy(ctx context.Context, ingester content.Ingester, provider content.Provider, desc ocispec.Descriptor) error {
	if _, err := remotes.FetchHandler(ingester, &localFetcher{provider})(ctx, desc); err != nil {
		return err
	}
	return nil
}

type localFetcher struct {
	content.Provider
}

func (f *localFetcher) Fetch(ctx context.Context, desc ocispec.Descriptor) (io.ReadCloser, error) {
	r, err := f.Provider.ReaderAt(ctx, desc)
	if err != nil {
		return nil, err
	}
	return &rc{ReaderAt: r}, nil
}

type rc struct {
	content.ReaderAt
	offset int
}

func (r *rc) Read(b []byte) (int, error) {
	n, err := r.ReadAt(b, int64(r.offset))
	r.offset += n
	if n > 0 && err == io.EOF {
		err = nil
	}
	return n, err
}
