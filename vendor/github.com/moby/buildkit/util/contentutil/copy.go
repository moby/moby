package contentutil

import (
	"context"
	"io"
	"strings"
	"sync"

	"github.com/containerd/containerd/v2/core/content"
	"github.com/containerd/containerd/v2/core/images"
	cerrdefs "github.com/containerd/errdefs"
	"github.com/moby/buildkit/util/resolver/limited"
	"github.com/moby/buildkit/util/resolver/retryhandler"
	ocispecs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

type CopyInfo struct {
	Referrers content.ReferrersProvider
}

type CopyOption func(*CopyInfo) error

func WithReferrers(referrers content.ReferrersProvider) CopyOption {
	return func(ci *CopyInfo) error {
		ci.Referrers = referrers
		return nil
	}
}

func Copy(ctx context.Context, ingester content.Ingester, provider content.Provider, desc ocispecs.Descriptor, ref string, logger func([]byte)) error {
	ctx = RegisterContentPayloadTypes(ctx)
	if _, err := retryhandler.New(limited.FetchHandler(ingester, &localFetcher{provider}, ref), logger)(ctx, desc); err != nil {
		return err
	}
	return nil
}

type localFetcher struct {
	content.Provider
}

func (f *localFetcher) Fetch(ctx context.Context, desc ocispecs.Descriptor) (io.ReadCloser, error) {
	r, err := f.ReaderAt(ctx, desc)
	if err != nil {
		return nil, err
	}
	return &rc{ReaderAt: r}, nil
}

type rc struct {
	content.ReaderAt
	offset int64
}

func (r *rc) Read(b []byte) (int, error) {
	n, err := r.ReadAt(b, r.offset)
	r.offset += int64(n)
	if n > 0 && errors.Is(err, io.EOF) {
		err = nil
	}
	return n, err
}

func (r *rc) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case io.SeekStart:
		r.offset = offset
	case io.SeekCurrent:
		r.offset += offset
	case io.SeekEnd:
		r.offset = r.Size() - offset
	}
	return r.offset, nil
}
func CopyChain(ctx context.Context, ingester content.Ingester, provider content.Provider, desc ocispecs.Descriptor, opts ...CopyOption) error {
	return copyChain(ctx, ingester, provider, desc, &sync.Map{}, opts...)
}

func copyChain(ctx context.Context, ingester content.Ingester, provider content.Provider, desc ocispecs.Descriptor, visited *sync.Map, opts ...CopyOption) error {
	ci := &CopyInfo{}
	for _, o := range opts {
		if err := o(ci); err != nil {
			return err
		}
	}

	ctx = RegisterContentPayloadTypes(ctx)
	var m sync.Mutex
	manifestStack := []ocispecs.Descriptor{}

	filterHandler := images.HandlerFunc(func(ctx context.Context, desc ocispecs.Descriptor) ([]ocispecs.Descriptor, error) {
		if _, ok := visited.Load(desc.Digest); ok {
			return nil, images.ErrStopHandler
		}
		visited.Store(desc.Digest, struct{}{})
		switch desc.MediaType {
		case images.MediaTypeDockerSchema2Manifest, ocispecs.MediaTypeImageManifest,
			images.MediaTypeDockerSchema2ManifestList, ocispecs.MediaTypeImageIndex:
			m.Lock()
			manifestStack = append(manifestStack, desc)
			m.Unlock()
			return nil, images.ErrStopHandler
		default:
			return nil, nil
		}
	})
	handlers := []images.Handler{
		annotateDistributionSourceHandler(images.ChildrenHandler(provider), desc.Annotations),
		filterHandler,
		retryhandler.New(limited.FetchHandler(ingester, &localFetcher{provider}, ""), func(_ []byte) {}),
	}

	if err := images.Dispatch(ctx, images.Handlers(handlers...), nil, desc); err != nil {
		return errors.WithStack(err)
	}

	for i := len(manifestStack) - 1; i >= 0; i-- {
		desc := manifestStack[i]
		if err := Copy(ctx, ingester, provider, desc, "", nil); err != nil {
			return errors.WithStack(err)
		}
		if ci.Referrers != nil {
			referrers, err := ci.Referrers.Referrers(ctx, desc)
			if err != nil {
				if errors.Is(err, cerrdefs.ErrNotFound) {
					continue
				}
				return errors.WithStack(err)
			}
			for _, r := range referrers {
				if err := copyChain(ctx, ingester, provider, r, visited, opts...); err != nil {
					return errors.WithStack(err)
				}
			}
		}
	}

	return nil
}

func annotateDistributionSourceHandler(f images.HandlerFunc, basis map[string]string) images.HandlerFunc {
	return func(ctx context.Context, desc ocispecs.Descriptor) ([]ocispecs.Descriptor, error) {
		children, err := f(ctx, desc)
		if err != nil {
			return nil, err
		}

		// only add distribution source for the config or blob data descriptor
		switch desc.MediaType {
		case images.MediaTypeDockerSchema2Manifest, ocispecs.MediaTypeImageManifest,
			images.MediaTypeDockerSchema2ManifestList, ocispecs.MediaTypeImageIndex:
		default:
			return children, nil
		}

		for i := range children {
			child := children[i]

			for k, v := range basis {
				if !strings.HasPrefix(k, "containerd.io/distribution.source.") {
					continue
				}
				if child.Annotations != nil {
					if _, ok := child.Annotations[k]; ok {
						// don't override if already present
						continue
					}
				}

				if child.Annotations == nil {
					child.Annotations = map[string]string{}
				}
				child.Annotations[k] = v
			}

			children[i] = child
		}

		return children, nil
	}
}
