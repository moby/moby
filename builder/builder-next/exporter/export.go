package containerimage

import (
	"context"
	"fmt"

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

type Differ interface {
	EnsureLayer(ctx context.Context, key string) ([]layer.DiffID, error)
}

type Opt struct {
	ImageStore     image.Store
	ReferenceStore reference.Store
	Differ         Differ
}

type imageExporter struct {
	opt Opt
}

func New(opt Opt) (exporter.Exporter, error) {
	im := &imageExporter{opt: opt}
	return im, nil
}

func (e *imageExporter) Resolve(ctx context.Context, opt map[string]string) (exporter.ExporterInstance, error) {
	i := &imageExporterInstance{imageExporter: e}
	for k, v := range opt {
		switch k {
		case keyImageName:
			ref, err := distref.ParseNormalizedNamed(v)
			if err != nil {
				return nil, err
			}
			i.targetName = ref
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
	targetName distref.Named
	config     []byte
}

func (e *imageExporterInstance) Name() string {
	return "exporting to image"
}

func (e *imageExporterInstance) Export(ctx context.Context, ref cache.ImmutableRef, opt map[string][]byte) error {
	if config, ok := opt[exporterImageConfig]; ok {
		e.config = config
	}
	config := e.config

	layersDone := oneOffProgress(ctx, "exporting layers")

	if err := ref.Finalize(ctx); err != nil {
		return err
	}

	diffIDs, err := e.opt.Differ.EnsureLayer(ctx, ref.ID())
	if err != nil {
		return err
	}

	diffs := make([]digest.Digest, len(diffIDs))
	for i := range diffIDs {
		diffs[i] = digest.Digest(diffIDs[i])
	}

	layersDone(nil)

	if len(config) == 0 {
		var err error
		config, err = emptyImageConfig()
		if err != nil {
			return err
		}
	}

	history, err := parseHistoryFromConfig(config)
	if err != nil {
		return err
	}

	diffs, history = normalizeLayersAndHistory(diffs, history, ref)

	config, err = patchImageConfig(config, diffs, history)
	if err != nil {
		return err
	}

	configDigest := digest.FromBytes(config)

	configDone := oneOffProgress(ctx, fmt.Sprintf("writing image %s", configDigest))
	id, err := e.opt.ImageStore.Create(config)
	if err != nil {
		return configDone(err)
	}
	configDone(nil)

	if e.targetName != nil {
		if e.opt.ReferenceStore != nil {
			tagDone := oneOffProgress(ctx, "naming to "+e.targetName.String())

			if err := e.opt.ReferenceStore.AddTag(e.targetName, digest.Digest(id), true); err != nil {
				return tagDone(err)
			}
			tagDone(nil)
		}
	}

	return nil
}
