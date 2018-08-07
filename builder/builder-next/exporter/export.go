package containerimage

import (
	"context"
	"fmt"
	"strings"

	distref "github.com/docker/distribution/reference"
	"github.com/docker/docker/image"
	"github.com/docker/docker/layer"
	"github.com/docker/docker/reference"
	"github.com/moby/buildkit/cache"
	"github.com/moby/buildkit/exporter"
	digest "github.com/opencontainers/go-digest"
	"github.com/sirupsen/logrus"
)

const (
	keyImageName        = "name"
	exporterImageConfig = "containerimage.config"
)

// Differ can make a moby layer from a snapshot
type Differ interface {
	EnsureLayer(ctx context.Context, key string) ([]layer.DiffID, error)
}

// Opt defines a struct for creating new exporter
type Opt struct {
	ImageStore     image.Store
	ReferenceStore reference.Store
	Differ         Differ
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
	i := &imageExporterInstance{imageExporter: e}
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
		case exporterImageConfig:
			i.config = []byte(v)
		default:
			logrus.Warnf("image exporter: unknown option %s", k)
		}
	}
	return i, nil
}

type imageExporterInstance struct {
	*imageExporter
	targetNames []distref.Named
	config      []byte
}

func (e *imageExporterInstance) Name() string {
	return "exporting to image"
}

func (e *imageExporterInstance) Export(ctx context.Context, ref cache.ImmutableRef, opt map[string][]byte) (map[string]string, error) {
	if config, ok := opt[exporterImageConfig]; ok {
		e.config = config
	}
	config := e.config

	var diffs []digest.Digest
	if ref != nil {
		layersDone := oneOffProgress(ctx, "exporting layers")

		if err := ref.Finalize(ctx, true); err != nil {
			return nil, err
		}

		diffIDs, err := e.opt.Differ.EnsureLayer(ctx, ref.ID())
		if err != nil {
			return nil, err
		}

		diffs = make([]digest.Digest, len(diffIDs))
		for i := range diffIDs {
			diffs[i] = digest.Digest(diffIDs[i])
		}

		layersDone(nil)
	}

	if len(config) == 0 {
		var err error
		config, err = emptyImageConfig()
		if err != nil {
			return nil, err
		}
	}

	history, err := parseHistoryFromConfig(config)
	if err != nil {
		return nil, err
	}

	diffs, history = normalizeLayersAndHistory(diffs, history, ref)

	config, err = patchImageConfig(config, diffs, history)
	if err != nil {
		return nil, err
	}

	configDigest := digest.FromBytes(config)

	configDone := oneOffProgress(ctx, fmt.Sprintf("writing image %s", configDigest))
	id, err := e.opt.ImageStore.Create(config)
	if err != nil {
		return nil, configDone(err)
	}
	configDone(nil)

	if e.opt.ReferenceStore != nil {
		for _, targetName := range e.targetNames {
			tagDone := oneOffProgress(ctx, "naming to "+targetName.String())

			if err := e.opt.ReferenceStore.AddTag(targetName, digest.Digest(id), true); err != nil {
				return nil, tagDone(err)
			}
			tagDone(nil)
		}
	}

	return map[string]string{
		"containerimage.digest": id.String(),
	}, nil
}
