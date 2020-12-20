package imageutil

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/leases"
	"github.com/containerd/containerd/platforms"
	"github.com/containerd/containerd/reference"
	"github.com/containerd/containerd/remotes"
	"github.com/containerd/containerd/remotes/docker"
	"github.com/moby/buildkit/util/leaseutil"
	"github.com/moby/buildkit/util/resolver/retryhandler"
	digest "github.com/opencontainers/go-digest"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

type ContentCache interface {
	content.Ingester
	content.Provider
}

var leasesMu sync.Mutex
var leasesF []func(context.Context) error

func CancelCacheLeases() {
	leasesMu.Lock()
	for _, f := range leasesF {
		f(context.TODO())
	}
	leasesF = nil
	leasesMu.Unlock()
}

func AddLease(f func(context.Context) error) {
	leasesMu.Lock()
	leasesF = append(leasesF, f)
	leasesMu.Unlock()
}

func Config(ctx context.Context, str string, resolver remotes.Resolver, cache ContentCache, leaseManager leases.Manager, p *specs.Platform) (digest.Digest, []byte, error) {
	// TODO: fix buildkit to take interface instead of struct
	var platform platforms.MatchComparer
	if p != nil {
		platform = platforms.Only(*p)
	} else {
		platform = platforms.Default()
	}
	ref, err := reference.Parse(str)
	if err != nil {
		return "", nil, errors.WithStack(err)
	}

	if leaseManager != nil {
		ctx2, done, err := leaseutil.WithLease(ctx, leaseManager, leases.WithExpiration(5*time.Minute), leaseutil.MakeTemporary)
		if err != nil {
			return "", nil, errors.WithStack(err)
		}
		ctx = ctx2
		defer func() {
			// this lease is not deleted to allow other components to access manifest/config from cache. It will be deleted after 5 min deadline or on pruning inactive builder
			AddLease(done)
		}()
	}

	desc := specs.Descriptor{
		Digest: ref.Digest(),
	}
	if desc.Digest != "" {
		ra, err := cache.ReaderAt(ctx, desc)
		if err == nil {
			desc.Size = ra.Size()
			mt, err := DetectManifestMediaType(ra)
			if err == nil {
				desc.MediaType = mt
			}
		}
	}
	// use resolver if desc is incomplete
	if desc.MediaType == "" {
		_, desc, err = resolver.Resolve(ctx, ref.String())
		if err != nil {
			return "", nil, err
		}
	}

	fetcher, err := resolver.Fetcher(ctx, ref.String())
	if err != nil {
		return "", nil, err
	}

	if desc.MediaType == images.MediaTypeDockerSchema1Manifest {
		return readSchema1Config(ctx, ref.String(), desc, fetcher, cache)
	}

	children := childrenConfigHandler(cache, platform)

	handlers := []images.Handler{
		retryhandler.New(remotes.FetchHandler(cache, fetcher), nil),
		children,
	}
	if err := images.Dispatch(ctx, images.Handlers(handlers...), nil, desc); err != nil {
		return "", nil, err
	}
	config, err := images.Config(ctx, cache, desc, platform)
	if err != nil {
		return "", nil, err
	}

	dt, err := content.ReadBlob(ctx, cache, config)
	if err != nil {
		return "", nil, err
	}

	return desc.Digest, dt, nil
}

func childrenConfigHandler(provider content.Provider, platform platforms.MatchComparer) images.HandlerFunc {
	return func(ctx context.Context, desc specs.Descriptor) ([]specs.Descriptor, error) {
		var descs []specs.Descriptor
		switch desc.MediaType {
		case images.MediaTypeDockerSchema2Manifest, specs.MediaTypeImageManifest:
			p, err := content.ReadBlob(ctx, provider, desc)
			if err != nil {
				return nil, err
			}

			// TODO(stevvooe): We just assume oci manifest, for now. There may be
			// subtle differences from the docker version.
			var manifest specs.Manifest
			if err := json.Unmarshal(p, &manifest); err != nil {
				return nil, err
			}

			descs = append(descs, manifest.Config)
		case images.MediaTypeDockerSchema2ManifestList, specs.MediaTypeImageIndex:
			p, err := content.ReadBlob(ctx, provider, desc)
			if err != nil {
				return nil, err
			}

			var index specs.Index
			if err := json.Unmarshal(p, &index); err != nil {
				return nil, err
			}

			if platform != nil {
				for _, d := range index.Manifests {
					if d.Platform == nil || platform.Match(*d.Platform) {
						descs = append(descs, d)
					}
				}
			} else {
				descs = append(descs, index.Manifests...)
			}
		case images.MediaTypeDockerSchema2Config, specs.MediaTypeImageConfig, docker.LegacyConfigMediaType:
			// childless data types.
			return nil, nil
		default:
			return nil, errors.Errorf("encountered unknown type %v; children may not be fetched", desc.MediaType)
		}

		return descs, nil
	}
}

// specs.MediaTypeImageManifest, // TODO: detect schema1/manifest-list
func DetectManifestMediaType(ra content.ReaderAt) (string, error) {
	// TODO: schema1

	dt := make([]byte, ra.Size())
	if _, err := ra.ReadAt(dt, 0); err != nil {
		return "", err
	}

	return DetectManifestBlobMediaType(dt)
}

func DetectManifestBlobMediaType(dt []byte) (string, error) {
	var mfst struct {
		MediaType string          `json:"mediaType"`
		Config    json.RawMessage `json:"config"`
	}

	if err := json.Unmarshal(dt, &mfst); err != nil {
		return "", err
	}

	if mfst.MediaType != "" {
		return mfst.MediaType, nil
	}
	if mfst.Config != nil {
		return images.MediaTypeDockerSchema2Manifest, nil
	}
	return images.MediaTypeDockerSchema2ManifestList, nil
}
