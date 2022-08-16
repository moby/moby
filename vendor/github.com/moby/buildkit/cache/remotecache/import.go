package remotecache

import (
	"context"
	"encoding/json"
	"io"
	"sync"
	"time"

	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/images"
	v1 "github.com/moby/buildkit/cache/remotecache/v1"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/solver"
	"github.com/moby/buildkit/util/imageutil"
	"github.com/moby/buildkit/worker"
	digest "github.com/opencontainers/go-digest"
	ocispecs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
)

// ResolveCacheImporterFunc returns importer and descriptor.
type ResolveCacheImporterFunc func(ctx context.Context, g session.Group, attrs map[string]string) (Importer, ocispecs.Descriptor, error)

type Importer interface {
	Resolve(ctx context.Context, desc ocispecs.Descriptor, id string, w worker.Worker) (solver.CacheManager, error)
}

type DistributionSourceLabelSetter interface {
	SetDistributionSourceLabel(context.Context, digest.Digest) error
	SetDistributionSourceAnnotation(desc ocispecs.Descriptor) ocispecs.Descriptor
}

func NewImporter(provider content.Provider) Importer {
	return &contentCacheImporter{provider: provider}
}

type contentCacheImporter struct {
	provider content.Provider
}

func (ci *contentCacheImporter) Resolve(ctx context.Context, desc ocispecs.Descriptor, id string, w worker.Worker) (solver.CacheManager, error) {
	dt, err := readBlob(ctx, ci.provider, desc)
	if err != nil {
		return nil, err
	}

	var mfst ocispecs.Index
	if err := json.Unmarshal(dt, &mfst); err != nil {
		return nil, err
	}

	allLayers := v1.DescriptorProvider{}

	var configDesc ocispecs.Descriptor

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

	if dsls, ok := ci.provider.(DistributionSourceLabelSetter); ok {
		for dgst, l := range allLayers {
			err := dsls.SetDistributionSourceLabel(ctx, dgst)
			_ = err // error ignored because layer may not exist
			l.Descriptor = dsls.SetDistributionSourceAnnotation(l.Descriptor)
			allLayers[dgst] = l
		}
	}

	if configDesc.Digest == "" {
		return ci.importInlineCache(ctx, dt, id, w)
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
	return solver.NewCacheManager(ctx, id, keysStorage, resultStorage), nil
}

func readBlob(ctx context.Context, provider content.Provider, desc ocispecs.Descriptor) ([]byte, error) {
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
	return dt, errors.WithStack(err)
}

func (ci *contentCacheImporter) importInlineCache(ctx context.Context, dt []byte, id string, w worker.Worker) (solver.CacheManager, error) {
	m := map[digest.Digest][]byte{}

	if err := ci.allDistributionManifests(ctx, dt, m); err != nil {
		return nil, err
	}

	var mu sync.Mutex
	var cMap = map[digest.Digest]*v1.CacheChains{}

	eg, ctx := errgroup.WithContext(ctx)
	for dgst, dt := range m {
		func(dgst digest.Digest, dt []byte) {
			eg.Go(func() error {
				var m ocispecs.Manifest

				if err := json.Unmarshal(dt, &m); err != nil {
					return errors.WithStack(err)
				}

				if m.Config.Digest == "" || len(m.Layers) == 0 {
					return nil
				}

				if dsls, ok := ci.provider.(DistributionSourceLabelSetter); ok {
					for i, l := range m.Layers {
						err := dsls.SetDistributionSourceLabel(ctx, l.Digest)
						_ = err // error ignored because layer may not exist
						m.Layers[i] = dsls.SetDistributionSourceAnnotation(l)
					}
				}

				p, err := content.ReadBlob(ctx, ci.provider, m.Config)
				if err != nil {
					return errors.WithStack(err)
				}

				var img image

				if err := json.Unmarshal(p, &img); err != nil {
					return errors.WithStack(err)
				}

				if len(img.Rootfs.DiffIDs) != len(m.Layers) {
					logrus.Warnf("invalid image with mismatching manifest and config")
					return nil
				}

				if img.Cache == nil {
					return nil
				}

				var config v1.CacheConfig
				if err := json.Unmarshal(img.Cache, &config.Records); err != nil {
					return errors.WithStack(err)
				}

				createdDates, createdMsg, err := parseCreatedLayerInfo(img)
				if err != nil {
					return err
				}

				layers := v1.DescriptorProvider{}
				for i, m := range m.Layers {
					if m.Annotations == nil {
						m.Annotations = map[string]string{}
					}
					if createdAt := createdDates[i]; createdAt != "" {
						m.Annotations["buildkit/createdat"] = createdAt
					}
					if createdBy := createdMsg[i]; createdBy != "" {
						m.Annotations["buildkit/description"] = createdBy
					}
					m.Annotations["containerd.io/uncompressed"] = img.Rootfs.DiffIDs[i].String()
					layers[m.Digest] = v1.DescriptorProviderPair{
						Descriptor: m,
						Provider:   ci.provider,
					}
					config.Layers = append(config.Layers, v1.CacheLayer{
						Blob:        m.Digest,
						ParentIndex: i - 1,
					})
				}

				dt, err = json.Marshal(config)
				if err != nil {
					return errors.WithStack(err)
				}
				cc := v1.NewCacheChains()
				if err := v1.ParseConfig(config, layers, cc); err != nil {
					return err
				}
				mu.Lock()
				cMap[dgst] = cc
				mu.Unlock()
				return nil
			})
		}(dgst, dt)
	}

	if err := eg.Wait(); err != nil {
		return nil, err
	}

	cms := make([]solver.CacheManager, 0, len(cMap))

	for _, cc := range cMap {
		keysStorage, resultStorage, err := v1.NewCacheKeyStorage(cc, w)
		if err != nil {
			return nil, err
		}
		cms = append(cms, solver.NewCacheManager(ctx, id, keysStorage, resultStorage))
	}

	return solver.NewCombinedCacheManager(cms, nil), nil
}

func (ci *contentCacheImporter) allDistributionManifests(ctx context.Context, dt []byte, m map[digest.Digest][]byte) error {
	mt, err := imageutil.DetectManifestBlobMediaType(dt)
	if err != nil {
		return err
	}

	switch mt {
	case images.MediaTypeDockerSchema2Manifest, ocispecs.MediaTypeImageManifest:
		m[digest.FromBytes(dt)] = dt
	case images.MediaTypeDockerSchema2ManifestList, ocispecs.MediaTypeImageIndex:
		var index ocispecs.Index
		if err := json.Unmarshal(dt, &index); err != nil {
			return errors.WithStack(err)
		}

		for _, d := range index.Manifests {
			if _, ok := m[d.Digest]; ok {
				continue
			}
			p, err := content.ReadBlob(ctx, ci.provider, d)
			if err != nil {
				return errors.WithStack(err)
			}
			if err := ci.allDistributionManifests(ctx, p, m); err != nil {
				return err
			}
		}
	}

	return nil
}

type image struct {
	Rootfs struct {
		DiffIDs []digest.Digest `json:"diff_ids"`
	} `json:"rootfs"`
	Cache   []byte `json:"moby.buildkit.cache.v0"`
	History []struct {
		Created    *time.Time `json:"created,omitempty"`
		CreatedBy  string     `json:"created_by,omitempty"`
		EmptyLayer bool       `json:"empty_layer,omitempty"`
	} `json:"history,omitempty"`
}

func parseCreatedLayerInfo(img image) ([]string, []string, error) {
	dates := make([]string, 0, len(img.Rootfs.DiffIDs))
	createdBy := make([]string, 0, len(img.Rootfs.DiffIDs))
	for _, h := range img.History {
		if !h.EmptyLayer {
			str := ""
			if h.Created != nil {
				dt, err := h.Created.MarshalText()
				if err != nil {
					return nil, nil, err
				}
				str = string(dt)
			}
			dates = append(dates, str)
			createdBy = append(createdBy, h.CreatedBy)
		}
	}
	return dates, createdBy, nil
}
