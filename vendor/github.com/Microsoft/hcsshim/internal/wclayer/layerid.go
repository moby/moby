//go:build windows

package wclayer

import (
	"context"
	"go.opentelemetry.io/otel/attribute"
	"path/filepath"

	"github.com/Microsoft/go-winio/pkg/guid"
	"github.com/Microsoft/hcsshim/internal/ot"
)

// LayerID returns the layer ID of a layer on disk.
func LayerID(ctx context.Context, path string) (_ guid.GUID, err error) {
	title := "hcsshim::LayerID"
	ctx, span := ot.StartSpan(ctx, title)
	defer span.End()
	defer func() { ot.SetSpanStatus(span, err) }()
	span.SetAttributes(attribute.String("path", path))

	_, file := filepath.Split(path)
	return NameToGuid(ctx, file)
}
