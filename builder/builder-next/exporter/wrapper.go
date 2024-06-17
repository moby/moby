package exporter

import (
	"context"
	"strings"

	"github.com/docker/docker/builder/builder-next/exporter/overrides"
	"github.com/moby/buildkit/exporter"
	"github.com/moby/buildkit/exporter/containerimage/exptypes"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

type ImageExportedByBuildkit = func(ctx context.Context, id string, desc ocispec.Descriptor) error

// Wraps the containerimage exporter's Resolve method to apply moby-specific
// overrides to the exporter attributes.
type imageExporterMobyWrapper struct {
	exp      exporter.Exporter
	callback ImageExportedByBuildkit
}

// NewWrapper returns an exporter wrapper that applies moby specific attributes
// and hooks the export process.
func NewWrapper(exp exporter.Exporter, callback ImageExportedByBuildkit) (exporter.Exporter, error) {
	return &imageExporterMobyWrapper{exp: exp, callback: callback}, nil
}

// Resolve applies moby specific attributes to the request.
func (e *imageExporterMobyWrapper) Resolve(ctx context.Context, id int, exporterAttrs map[string]string) (exporter.ExporterInstance, error) {
	if exporterAttrs == nil {
		exporterAttrs = make(map[string]string)
	}
	reposAndTags, err := overrides.SanitizeRepoAndTags(strings.Split(exporterAttrs[string(exptypes.OptKeyName)], ","))
	if err != nil {
		return nil, err
	}
	exporterAttrs[string(exptypes.OptKeyName)] = strings.Join(reposAndTags, ",")
	exporterAttrs[string(exptypes.OptKeyUnpack)] = "true"
	if _, has := exporterAttrs[string(exptypes.OptKeyDanglingPrefix)]; !has {
		exporterAttrs[string(exptypes.OptKeyDanglingPrefix)] = "moby-dangling"
	}

	inst, err := e.exp.Resolve(ctx, id, exporterAttrs)
	if err != nil {
		return nil, err
	}

	return &imageExporterInstanceWrapper{ExporterInstance: inst, callback: e.callback}, nil
}

type imageExporterInstanceWrapper struct {
	exporter.ExporterInstance
	callback ImageExportedByBuildkit
}

func (i *imageExporterInstanceWrapper) Export(ctx context.Context, src *exporter.Source, inlineCache exptypes.InlineCache, sessionID string) (map[string]string, exporter.DescriptorReference, error) {
	out, ref, err := i.ExporterInstance.Export(ctx, src, inlineCache, sessionID)
	if err != nil {
		return out, ref, err
	}

	desc := ref.Descriptor()
	imageID := out[exptypes.ExporterImageDigestKey]
	if i.callback != nil {
		i.callback(ctx, imageID, desc)
	}
	return out, ref, nil
}
