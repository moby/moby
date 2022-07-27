package overrides

import (
	"context"
	"strings"

	"github.com/moby/buildkit/exporter"
)

// Wraps the containerimage exporter's Resolve method to apply moby-specific
// overrides to the exporter attributes.
type imageExporterMobyWrapper struct {
	exp exporter.Exporter
}

func NewExporterWrapper(exp exporter.Exporter) (exporter.Exporter, error) {
	return &imageExporterMobyWrapper{exp: exp}, nil
}

// Resolve applies moby specific attributes to the request.
func (e *imageExporterMobyWrapper) Resolve(ctx context.Context, exporterAttrs map[string]string) (exporter.ExporterInstance, error) {
	if exporterAttrs == nil {
		exporterAttrs = make(map[string]string)
	}
	reposAndTags, err := SanitizeRepoAndTags(strings.Split(exporterAttrs["name"], ","))
	if err != nil {
		return nil, err
	}
	exporterAttrs["name"] = strings.Join(reposAndTags, ",")
	exporterAttrs["unpack"] = "true"

	return e.exp.Resolve(ctx, exporterAttrs)
}
