package mobyexporter

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	distref "github.com/distribution/reference"
	"github.com/moby/buildkit/exporter"
	"github.com/moby/buildkit/exporter/containerimage/exptypes"
	"github.com/opencontainers/go-digest"
	"github.com/pkg/errors"

	"github.com/docker/docker/image"
	"github.com/docker/docker/layer"
)

const (
	keyImageName = "name"
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

func (e *imageExporter) Resolve(ctx context.Context, opt map[string]string) (exporter.ExporterInstance, error) {
	i := &imageExporterInstance{
		imageExporter: e,
	}
	for k, v := range opt {
		switch k {
		case keyImageName:
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
	targetNames []distref.Named
	meta        map[string][]byte
}

func (e *imageExporterInstance) Name() string {
	return "exporting to image"
}

func (e *imageExporterInstance) Config() *exporter.Config {
	return exporter.NewConfig()
}

func (e *imageExporterInstance) Export(ctx context.Context, inp *exporter.Source, sessionID string) (map[string]string, exporter.DescriptorReference, error) {
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
		platformsBytes, ok := inp.Metadata[exptypes.ExporterPlatformsKey]
		if !ok {
			return nil, nil, fmt.Errorf("cannot export image, missing platforms mapping")
		}
		var p exptypes.Platforms
		if err := json.Unmarshal(platformsBytes, &p); err != nil {
			return nil, nil, errors.Wrapf(err, "failed to parse platforms passed to exporter")
		}
		if len(p.Platforms) != len(inp.Refs) {
			return nil, nil, errors.Errorf("number of platforms does not match references %d %d", len(p.Platforms), len(inp.Refs))
		}
		config = inp.Metadata[fmt.Sprintf("%s/%s", exptypes.ExporterImageConfigKey, p.Platforms[0].ID)]
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

	config, err = patchImageConfig(config, diffs, history, inp.Metadata[exptypes.ExporterInlineCache])
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
