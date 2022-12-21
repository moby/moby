package registry

import (
	"context"
	"encoding/json"

	"github.com/moby/buildkit/cache/remotecache"
	v1 "github.com/moby/buildkit/cache/remotecache/v1"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/solver"
	"github.com/moby/buildkit/util/compression"
	digest "github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

func ResolveCacheExporterFunc() remotecache.ResolveCacheExporterFunc {
	return func(ctx context.Context, _ session.Group, _ map[string]string) (remotecache.Exporter, error) {
		return NewExporter(), nil
	}
}

func NewExporter() remotecache.Exporter {
	cc := v1.NewCacheChains()
	return &exporter{CacheExporterTarget: cc, chains: cc}
}

type exporter struct {
	solver.CacheExporterTarget
	chains *v1.CacheChains
}

func (*exporter) Name() string {
	return "exporting inline cache"
}

func (ce *exporter) Config() remotecache.Config {
	return remotecache.Config{
		Compression: compression.New(compression.Default),
	}
}

func (ce *exporter) Finalize(ctx context.Context) (map[string]string, error) {
	return nil, nil
}

func (ce *exporter) reset() {
	cc := v1.NewCacheChains()
	ce.CacheExporterTarget = cc
	ce.chains = cc
}

func (ce *exporter) ExportForLayers(ctx context.Context, layers []digest.Digest) ([]byte, error) {
	config, descs, err := ce.chains.Marshal(ctx)
	if err != nil {
		return nil, err
	}

	descs2 := map[digest.Digest]v1.DescriptorProviderPair{}
	for _, k := range layers {
		if v, ok := descs[k]; ok {
			descs2[k] = v
			continue
		}
		// fallback for uncompressed digests
		for _, v := range descs {
			if uc := v.Descriptor.Annotations["containerd.io/uncompressed"]; uc == string(k) {
				descs2[v.Descriptor.Digest] = v
			}
		}
	}

	cc := v1.NewCacheChains()
	if err := v1.ParseConfig(*config, descs2, cc); err != nil {
		return nil, err
	}

	cfg, _, err := cc.Marshal(ctx)
	if err != nil {
		return nil, err
	}

	if len(cfg.Layers) == 0 {
		logrus.Warn("failed to match any cache with layers")
		return nil, nil
	}

	// reorder layers based on the order in the image
	blobIndexes := make(map[digest.Digest]int, len(layers))
	for i, blob := range layers {
		blobIndexes[blob] = i
	}

	for i, r := range cfg.Records {
		for j, rr := range r.Results {
			resultBlobs := layerToBlobs(rr.LayerIndex, cfg.Layers)
			// match being true means the result is in the same order as the image
			var match bool
			if len(resultBlobs) <= len(layers) {
				match = true
				for k, resultBlob := range resultBlobs {
					layerBlob := layers[k]
					if resultBlob != layerBlob {
						match = false
						break
					}
				}
			}
			if match {
				// The layers of the result are in the same order as the image, so we can
				// specify it just using the CacheResult struct and specifying LayerIndex
				// as the top-most layer of the result.
				rr.LayerIndex = len(resultBlobs) - 1
				r.Results[j] = rr
			} else {
				// The layers of the result are not in the same order as the image, so we
				// have to use ChainedResult to specify each layer of the result individually.
				chainedResult := v1.ChainedResult{}
				for _, resultBlob := range resultBlobs {
					idx, ok := blobIndexes[resultBlob]
					if !ok {
						return nil, errors.Errorf("failed to find blob %s in layers", resultBlob)
					}
					chainedResult.LayerIndexes = append(chainedResult.LayerIndexes, idx)
				}
				r.Results[j] = v1.CacheResult{}
				r.ChainedResults = append(r.ChainedResults, chainedResult)
			}
			// remove any CacheResults that had to be converted to the ChainedResult format.
			var filteredResults []v1.CacheResult
			for _, rr := range r.Results {
				if rr != (v1.CacheResult{}) {
					filteredResults = append(filteredResults, rr)
				}
			}
			r.Results = filteredResults
			cfg.Records[i] = r
		}
	}

	dt, err := json.Marshal(cfg.Records)
	if err != nil {
		return nil, err
	}
	ce.reset()

	return dt, nil
}

func layerToBlobs(idx int, layers []v1.CacheLayer) []digest.Digest {
	var ds []digest.Digest
	for idx != -1 {
		layer := layers[idx]
		ds = append(ds, layer.Blob)
		idx = layer.ParentIndex
	}
	// reverse so they go lowest to highest
	for i, j := 0, len(ds)-1; i < j; i, j = i+1, j-1 {
		ds[i], ds[j] = ds[j], ds[i]
	}
	return ds
}
