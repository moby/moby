package contentutil

import (
	"context"
	"net/http"

	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/remotes"
	"github.com/containerd/containerd/remotes/docker"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
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
		pusher: pusher,
	}, nil
}

type ingester struct {
	pusher remotes.Pusher
}

func (w *ingester) Writer(ctx context.Context, opts ...content.WriterOpt) (content.Writer, error) {
	var wo content.WriterOpts
	for _, o := range opts {
		if err := o(&wo); err != nil {
			return nil, err
		}
	}
	return w.pusher.Push(ctx, wo.Desc)
}
