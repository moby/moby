package cacheimport

import (
	"encoding/json"

	cacheimporttypes "github.com/moby/buildkit/cache/remotecache/v1/types"
	"github.com/moby/buildkit/solver"
	"github.com/moby/buildkit/util/contentutil"
	ocispecs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

func Parse(configJSON []byte, provider DescriptorProvider, t solver.CacheExporterTarget) error {
	var config cacheimporttypes.CacheConfig
	if err := json.Unmarshal(configJSON, &config); err != nil {
		return errors.WithStack(err)
	}

	return ParseConfig(config, provider, t)
}

func ParseConfig(config cacheimporttypes.CacheConfig, provider DescriptorProvider, t solver.CacheExporterTarget) error {
	cache := map[int]solver.CacheExporterRecord{}

	for i := range config.Records {
		if _, err := parseRecord(config, i, provider, t, cache); err != nil {
			return err
		}
	}
	return nil
}

func parseRecord(cc cacheimporttypes.CacheConfig, idx int, provider DescriptorProvider, t solver.CacheExporterTarget, cache map[int]solver.CacheExporterRecord) (solver.CacheExporterRecord, error) {
	if r, ok := cache[idx]; ok {
		if r == nil {
			return nil, errors.Errorf("invalid looping record")
		}
		return r, nil
	}

	cache[idx] = nil
	if idx < 0 || idx >= len(cc.Records) {
		return nil, errors.Errorf("invalid record ID: %d", idx)
	}
	rec := cc.Records[idx]

	links := make([][]solver.CacheLink, len(rec.Inputs))

	for i, inputs := range rec.Inputs {
		if len(inputs) == 0 {
			return nil, errors.Errorf("invalid empty input for record %d", idx)
		}
		links[i] = make([]solver.CacheLink, len(inputs))
		for j, inp := range inputs {
			src, err := parseRecord(cc, inp.LinkIndex, provider, t, cache)
			if err != nil {
				return nil, err
			}
			links[i][j] = solver.CacheLink{
				Selector: inp.Selector,
				Src:      src,
			}
		}
	}

	results := make([]solver.CacheExportResult, 0, len(rec.Results))
	for _, res := range rec.Results {
		visited := map[int]struct{}{}
		remote, err := getRemoteChain(cc.Layers, res.LayerIndex, provider, visited)
		if err != nil {
			return nil, err
		}
		if remote != nil {
			results = append(results, solver.CacheExportResult{
				CreatedAt: res.CreatedAt,
				Result:    remote,
			})
		}
	}
	for _, res := range rec.ChainedResults {
		remote := &solver.Remote{}
		mp := contentutil.NewMultiProvider(nil)
		for _, diff := range res.LayerIndexes {
			if diff < 0 || diff >= len(cc.Layers) {
				return nil, errors.Errorf("invalid layer index %d", diff)
			}

			l := cc.Layers[diff]

			descPair, ok := provider[l.Blob]
			if !ok {
				remote = nil
				break
			}

			remote.Descriptors = append(remote.Descriptors, descPair.Descriptor)
			mp.Add(descPair.Descriptor.Digest, descPair)
		}
		if remote != nil {
			remote.Provider = mp
			results = append(results, solver.CacheExportResult{
				CreatedAt: res.CreatedAt,
				Result:    remote,
			})
		}
	}

	r, _, err := t.Add(rec.Digest, links, results)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to add record %d", idx)
	}
	cache[idx] = r
	return r, nil
}

func getRemoteChain(layers []cacheimporttypes.CacheLayer, idx int, provider DescriptorProvider, visited map[int]struct{}) (*solver.Remote, error) {
	if _, ok := visited[idx]; ok {
		return nil, errors.Errorf("invalid looping layer")
	}
	visited[idx] = struct{}{}

	if idx < 0 || idx >= len(layers) {
		return nil, errors.Errorf("invalid layer index %d", idx)
	}

	l := layers[idx]

	descPair, ok := provider[l.Blob]
	if !ok {
		return nil, nil
	}

	var r *solver.Remote
	if l.ParentIndex != -1 {
		var err error
		r, err = getRemoteChain(layers, l.ParentIndex, provider, visited)
		if err != nil {
			return nil, err
		}
		if r == nil {
			return nil, nil
		}
		r.Descriptors = append(r.Descriptors, descPair.Descriptor)
		mp := contentutil.NewMultiProvider(r.Provider)
		mp.Add(descPair.Descriptor.Digest, descPair)
		r.Provider = mp
		return r, nil
	}
	return &solver.Remote{
		Descriptors: []ocispecs.Descriptor{descPair.Descriptor},
		Provider:    descPair,
	}, nil
}
