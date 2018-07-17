package remotecache

import (
	"context"
	"encoding/json"
	"io"

	"github.com/containerd/containerd/content"
	v1 "github.com/moby/buildkit/cache/remotecache/v1"
	"github.com/moby/buildkit/solver"
	"github.com/moby/buildkit/worker"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

// ResolveCacheImporterFunc returns importer and descriptor.
// Currently typ needs to be an empty string.
type ResolveCacheImporterFunc func(ctx context.Context, typ, ref string) (Importer, ocispec.Descriptor, error)

type Importer interface {
	Resolve(ctx context.Context, desc ocispec.Descriptor, id string, w worker.Worker) (solver.CacheManager, error)
}

func NewImporter(provider content.Provider) Importer {
	return &contentCacheImporter{provider: provider}
}

type contentCacheImporter struct {
	provider content.Provider
}

func (ci *contentCacheImporter) Resolve(ctx context.Context, desc ocispec.Descriptor, id string, w worker.Worker) (solver.CacheManager, error) {
	dt, err := readBlob(ctx, ci.provider, desc)
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
			Provider:   ci.provider,
		}
	}

	if configDesc.Digest == "" {
		return nil, errors.Errorf("invalid build cache from %+v", desc)
	}

	dt, err = readBlob(ctx, ci.provider, configDesc)
	if err != nil {
		return nil, err
	}

	cc := v1.NewCacheChains()
	if err := v1.Parse(dt, allLayers, cc); err != nil {
		return nil, err
	}

	keysStorage, resultStorage, err := v1.NewCacheKeyStorage(cc, w)
	if err != nil {
		return nil, err
	}
	return solver.NewCacheManager(id, keysStorage, resultStorage), nil
}

func readBlob(ctx context.Context, provider content.Provider, desc ocispec.Descriptor) ([]byte, error) {
	maxBlobSize := int64(1 << 20)
	if desc.Size > maxBlobSize {
		return nil, errors.Errorf("blob %s is too large (%d > %d)", desc.Digest, desc.Size, maxBlobSize)
	}
	dt, err := content.ReadBlob(ctx, provider, desc)
	if err != nil {
		// NOTE: even if err == EOF, we might have got expected dt here.
		// For instance, http.Response.Body is known to return non-zero bytes with EOF.
		if err == io.EOF {
			if dtDigest := desc.Digest.Algorithm().FromBytes(dt); dtDigest != desc.Digest {
				err = errors.Wrapf(err, "got EOF, expected %s (%d bytes), got %s (%d bytes)",
					desc.Digest, desc.Size, dtDigest, len(dt))
			} else {
				err = nil
			}
		}
	}
	return dt, err
}
