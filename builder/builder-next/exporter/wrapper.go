package exporter

import (
	"context"
	"strings"

	"github.com/docker/docker/builder/builder-next/exporter/overrides"
	"github.com/moby/buildkit/exporter"
	"github.com/moby/buildkit/exporter/containerimage/exptypes"
)

// Wraps the containerimage exporter's Resolve method to apply moby-specific
// overrides to the exporter attributes.
type imageExporterMobyWrapper struct {
	exp exporter.Exporter
}

// NewWrapper returns an exporter wrapper that applies moby specific attributes
// and hooks the export event.
func NewWrapper(exp exporter.Exporter) (exporter.Exporter, error) {
	return &imageExporterMobyWrapper{exp: exp}, nil
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

	return e.exp.Resolve(ctx, id, exporterAttrs)
}
