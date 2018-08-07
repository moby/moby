package registry

import (
	"context"
	"time"

	"github.com/containerd/containerd/remotes"
	"github.com/containerd/containerd/remotes/docker"
	"github.com/moby/buildkit/cache/remotecache"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/session/auth"
	"github.com/moby/buildkit/util/contentutil"
	"github.com/moby/buildkit/util/tracing"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

func ResolveCacheExporterFunc(sm *session.Manager) remotecache.ResolveCacheExporterFunc {
	return func(ctx context.Context, typ, ref string) (remotecache.Exporter, error) {
		if typ != "" {
			return nil, errors.Errorf("unsupported cache exporter type: %s", typ)
		}
		remote := newRemoteResolver(ctx, sm)
		pusher, err := remote.Pusher(ctx, ref)
		if err != nil {
			return nil, err
		}
		return remotecache.NewExporter(contentutil.FromPusher(pusher)), nil
	}
}

func ResolveCacheImporterFunc(sm *session.Manager) remotecache.ResolveCacheImporterFunc {
	return func(ctx context.Context, typ, ref string) (remotecache.Importer, specs.Descriptor, error) {
		if typ != "" {
			return nil, specs.Descriptor{}, errors.Errorf("unsupported cache importer type: %s", typ)
		}
		remote := newRemoteResolver(ctx, sm)
		xref, desc, err := remote.Resolve(ctx, ref)
		if err != nil {
			return nil, specs.Descriptor{}, err
		}
		fetcher, err := remote.Fetcher(ctx, xref)
		if err != nil {
			return nil, specs.Descriptor{}, err
		}
		return remotecache.NewImporter(contentutil.FromFetcher(fetcher)), desc, nil
	}
}

func newRemoteResolver(ctx context.Context, sm *session.Manager) remotes.Resolver {
	return docker.NewResolver(docker.ResolverOptions{
		Client:      tracing.DefaultClient,
		Credentials: getCredentialsFunc(ctx, sm),
	})
}

func getCredentialsFunc(ctx context.Context, sm *session.Manager) func(string) (string, string, error) {
	id := session.FromContext(ctx)
	if id == "" {
		return nil
	}
	return func(host string) (string, string, error) {
		timeoutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		caller, err := sm.Get(timeoutCtx, id)
		if err != nil {
			return "", "", err
		}

		return auth.CredentialsFunc(context.TODO(), caller)(host)
	}
}
