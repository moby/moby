package remotecache

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/remotes"
	"github.com/containerd/containerd/remotes/docker"
	v1 "github.com/moby/buildkit/cache/remotecache/v1"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/session/auth"
	"github.com/moby/buildkit/solver"
	"github.com/moby/buildkit/util/contentutil"
	"github.com/moby/buildkit/worker"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

type ImportOpt struct {
	SessionManager *session.Manager
	Worker         worker.Worker // TODO: remove. This sets the worker where the cache is imported to. Should be passed on load instead.
}

func NewCacheImporter(opt ImportOpt) *CacheImporter {
	return &CacheImporter{opt: opt}
}

type CacheImporter struct {
	opt ImportOpt
}

func (ci *CacheImporter) getCredentialsFromSession(ctx context.Context) func(string) (string, string, error) {
	id := session.FromContext(ctx)
	if id == "" {
		return nil
	}

	return func(host string) (string, string, error) {
		timeoutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		caller, err := ci.opt.SessionManager.Get(timeoutCtx, id)
		if err != nil {
			return "", "", err
		}

		return auth.CredentialsFunc(context.TODO(), caller)(host)
	}
}

func (ci *CacheImporter) Resolve(ctx context.Context, ref string) (solver.CacheManager, error) {
	resolver := docker.NewResolver(docker.ResolverOptions{
		Client:      http.DefaultClient,
		Credentials: ci.getCredentialsFromSession(ctx),
	})

	ref, desc, err := resolver.Resolve(ctx, ref)
	if err != nil {
		return nil, err
	}

	fetcher, err := resolver.Fetcher(ctx, ref)
	if err != nil {
		return nil, err
	}

	b := contentutil.NewBuffer()

	if _, err := remotes.FetchHandler(b, fetcher)(ctx, desc); err != nil {
		return nil, err
	}

	dt, err := content.ReadBlob(ctx, b, desc)
	if err != nil {
		return nil, err
	}

	var mfst ocispec.Index
	if err := json.Unmarshal(dt, &mfst); err != nil {
		return nil, err
	}

	allLayers := v1.DescriptorProvider{}

	var configDesc ocispec.Descriptor

	for _, m := range mfst.Manifests {
		if m.MediaType == v1.CacheConfigMediaTypeV0 {
			configDesc = m
			continue
		}
		allLayers[m.Digest] = v1.DescriptorProviderPair{
			Descriptor: m,
			Provider:   contentutil.FromFetcher(fetcher, m),
		}
	}

	if configDesc.Digest == "" {
		return nil, errors.Errorf("invalid build cache from %s", ref)
	}

	if _, err := remotes.FetchHandler(b, fetcher)(ctx, configDesc); err != nil {
		return nil, err
	}

	dt, err = content.ReadBlob(ctx, b, configDesc)
	if err != nil {
		return nil, err
	}

	cc := v1.NewCacheChains()
	if err := v1.Parse(dt, allLayers, cc); err != nil {
		return nil, err
	}

	keysStorage, resultStorage, err := v1.NewCacheKeyStorage(cc, ci.opt.Worker)
	if err != nil {
		return nil, err
	}
	return solver.NewCacheManager(ref, keysStorage, resultStorage), nil
}
