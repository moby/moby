package contentutil

import (
	"context"
	"net/http"
	"sync"

	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/remotes"
	"github.com/containerd/containerd/remotes/docker"
	"github.com/docker/docker/pkg/locker"
	digest "github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

func ProviderFromRef(ref string) (ocispec.Descriptor, content.Provider, error) {
	remote := docker.NewResolver(docker.ResolverOptions{
		Client: http.DefaultClient,
	})

	name, desc, err := remote.Resolve(context.TODO(), ref)
	if err != nil {
		return ocispec.Descriptor{}, nil, err
	}

	fetcher, err := remote.Fetcher(context.TODO(), name)
	if err != nil {
		return ocispec.Descriptor{}, nil, err
	}
	return desc, FromFetcher(fetcher), nil
}

func IngesterFromRef(ref string) (content.Ingester, error) {
	remote := docker.NewResolver(docker.ResolverOptions{
		Client: http.DefaultClient,
	})

	pusher, err := remote.Pusher(context.TODO(), ref)
	if err != nil {
		return nil, err
	}

	return &ingester{
		locker: locker.New(),
		pusher: pusher,
	}, nil
}

type ingester struct {
	locker *locker.Locker
	pusher remotes.Pusher
}

func (w *ingester) Writer(ctx context.Context, opts ...content.WriterOpt) (content.Writer, error) {
	var wo content.WriterOpts
	for _, o := range opts {
		if err := o(&wo); err != nil {
			return nil, err
		}
	}
	if wo.Ref == "" {
		return nil, errors.Wrap(errdefs.ErrInvalidArgument, "ref must not be empty")
	}
	w.locker.Lock(wo.Ref)
	var once sync.Once
	unlock := func() {
		once.Do(func() {
			w.locker.Unlock(wo.Ref)
		})
	}
	writer, err := w.pusher.Push(ctx, wo.Desc)
	if err != nil {
		unlock()
		return nil, err
	}
	return &lockedWriter{unlock: unlock, Writer: writer}, nil
}

type lockedWriter struct {
	unlock func()
	content.Writer
}

func (w *lockedWriter) Commit(ctx context.Context, size int64, expected digest.Digest, opts ...content.Opt) error {
	err := w.Writer.Commit(ctx, size, expected, opts...)
	if err == nil {
		w.unlock()
	}
	return err
}

func (w *lockedWriter) Close() error {
	err := w.Writer.Close()
	w.unlock()
	return err
}
