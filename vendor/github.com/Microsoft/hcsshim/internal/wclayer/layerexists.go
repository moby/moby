//go:build windows

package wclayer

import (
	"context"
	"go.opentelemetry.io/otel/attribute"

	"github.com/Microsoft/hcsshim/internal/hcserror"
	"github.com/Microsoft/hcsshim/internal/ot"
)

// LayerExists will return true if a layer with the given id exists and is known
// to the system.
func LayerExists(ctx context.Context, path string) (_ bool, err error) {
	title := "hcsshim::LayerExists"
	ctx, span := ot.StartSpan(ctx, title) //nolint:ineffassign,staticcheck
	defer span.End()
	defer func() { ot.SetSpanStatus(span, err) }()
	span.SetAttributes(attribute.String("path", path))

	// Call the procedure itself.
	var exists uint32
	err = layerExists(&stdDriverInfo, path, &exists)
	if err != nil {
		return false, hcserror.New(err, title, "")
	}
	span.SetAttributes(attribute.Bool("layer-exists", exists != 0))
	return exists != 0, nil
}
