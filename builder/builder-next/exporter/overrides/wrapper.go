package overrides

import (
	"context"
	"strings"

	"github.com/moby/buildkit/exporter"
)

// TODO(vvoland): Use buildkit consts once they're public
// https://github.com/moby/buildkit/pull/3694
const (
	keyImageName      = "name"
	keyUnpack         = "unpack"
	keyDanglingPrefix = "dangling-name-prefix"
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
	reposAndTags, err := SanitizeRepoAndTags(strings.Split(exporterAttrs[keyImageName], ","))
	if err != nil {
		return nil, err
	}
	exporterAttrs[keyImageName] = strings.Join(reposAndTags, ",")
	exporterAttrs[keyUnpack] = "true"
	if _, has := exporterAttrs[keyDanglingPrefix]; !has {
		exporterAttrs[keyDanglingPrefix] = "moby-dangling"
	}

	return e.exp.Resolve(ctx, exporterAttrs)
}
