package mobyexporter

import (
	"context"
	"fmt"
	"strings"

	distref "github.com/distribution/reference"
	"github.com/docker/docker/image"
	"github.com/docker/docker/layer"
	"github.com/moby/buildkit/exporter"
	"github.com/moby/buildkit/exporter/containerimage/exptypes"
	"github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
)

// Differ can make a moby layer from a snapshot
type Differ interface {
	EnsureLayer(ctx context.Context, key string) ([]layer.DiffID, error)
}

type ImageTagger interface {
	TagImage(ctx context.Context, imageID image.ID, newTag distref.Named) error
}

// Opt defines a struct for creating new exporter
type Opt struct {
	ImageStore  image.Store
	Differ      Differ
	ImageTagger ImageTagger
}

type imageExporter struct {
	opt Opt
}

// New creates a new moby imagestore exporter
func New(opt Opt) (exporter.Exporter, error) {
	im := &imageExporter{opt: opt}
	return im, nil
}

func (e *imageExporter) Resolve(ctx context.Context, id int, opt map[string]string) (exporter.ExporterInstance, error) {
	i := &imageExporterInstance{
		imageExporter: e,
		id:            id,
	}
	for k, v := range opt {
		switch exptypes.ImageExporterOptKey(k) {
		case exptypes.OptKeyName:
			for _, v := range strings.Split(v, ",") {
				ref, err := distref.ParseNormalizedNamed(v)
				if err != nil {
					return nil, err
				}
				i.targetNames = append(i.targetNames, ref)
			}
		default:
			if i.meta == nil {
				i.meta = make(map[string][]byte)
			}
			i.meta[k] = []byte(v)
		}
	}
	return i, nil
}

type imageExporterInstance struct {
	*imageExporter
	id          int
	targetNames []distref.Named
	meta        map[string][]byte
}

func (e *imageExporterInstance) ID() int {
	return e.id
}

func (e *imageExporterInstance) Name() string {
	return "exporting to image"
}

func (e *imageExporterInstance) Config() *exporter.Config {
	return exporter.NewConfig()
}

func (e *imageExporterInstance) Export(ctx context.Context, inp *exporter.Source, inlineCache exptypes.InlineCache, sessionID string) (map[string]string, exporter.DescriptorReference, error) {
	if len(inp.Refs) > 1 {
		return nil, nil, fmt.Errorf("exporting multiple references to image store is currently unsupported")
	}

	ref := inp.Ref
	if ref != nil && len(inp.Refs) == 1 {
		return nil, nil, fmt.Errorf("invalid exporter input: Ref and Refs are mutually exclusive")
	}

	// only one loop
	for _, v := range inp.Refs {
		ref = v
	}

	var config []byte
	switch len(inp.Refs) {
	case 0:
		config = inp.Metadata[exptypes.ExporterImageConfigKey]
	case 1:
		ps, err := exptypes.ParsePlatforms(inp.Metadata)
		if err != nil {
			return nil, nil, fmt.Errorf("cannot export image, failed to parse platforms: %w", err)
		}
		if len(ps.Platforms) != len(inp.Refs) {
			return nil, nil, errors.Errorf("number of platforms does not match references %d %d", len(ps.Platforms), len(inp.Refs))
		}
		config = inp.Metadata[fmt.Sprintf("%s/%s", exptypes.ExporterImageConfigKey, ps.Platforms[0].ID)]
	}

	var diffs []digest.Digest
	if ref != nil {
		layersDone := oneOffProgress(ctx, "exporting layers")

		if err := ref.Finalize(ctx); err != nil {
			return nil, nil, layersDone(err)
		}

		if err := ref.Extract(ctx, nil); err != nil {
			return nil, nil, err
		}

		diffIDs, err := e.opt.Differ.EnsureLayer(ctx, ref.ID())
		if err != nil {
			return nil, nil, layersDone(err)
		}

		diffs = make([]digest.Digest, len(diffIDs))
		for i := range diffIDs {
			diffs[i] = digest.Digest(diffIDs[i])
		}

		_ = layersDone(nil)
	}

	if len(config) == 0 {
		var err error
		config, err = emptyImageConfig()
		if err != nil {
			return nil, nil, err
		}
	}

	history, err := parseHistoryFromConfig(config)
	if err != nil {
		return nil, nil, err
	}

	diffs, history = normalizeLayersAndHistory(diffs, history, ref)

	var inlineCacheEntry *exptypes.InlineCacheEntry
	if inlineCache != nil {
		inlineCacheResult, err := inlineCache(ctx)
		if err != nil {
			return nil, nil, err
		}
		if inlineCacheResult != nil {
			if ref != nil {
				inlineCacheEntry, _ = inlineCacheResult.FindRef(ref.ID())
			} else {
				inlineCacheEntry = inlineCacheResult.Ref
			}
		}
	}
	config, err = patchImageConfig(config, diffs, history, inlineCacheEntry)
	if err != nil {
		return nil, nil, err
	}

	configDigest := digest.FromBytes(config)

	configDone := oneOffProgress(ctx, fmt.Sprintf("writing image %s", configDigest))
	id, err := e.opt.ImageStore.Create(config)
	if err != nil {
		return nil, nil, configDone(err)
	}
	_ = configDone(nil)

	if e.opt.ImageTagger != nil {
		for _, targetName := range e.targetNames {
			tagDone := oneOffProgress(ctx, "naming to "+targetName.String())
			if err := e.opt.ImageTagger.TagImage(ctx, image.ID(digest.Digest(id)), targetName); err != nil {
				return nil, nil, tagDone(err)
			}
			_ = tagDone(nil)
		}
	}

	return map[string]string{
		exptypes.ExporterImageConfigDigestKey: configDigest.String(),
		exptypes.ExporterImageDigestKey:       id.String(),
	}, nil, nil
}
