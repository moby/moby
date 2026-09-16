//go:build windows

package wclayer

import (
	"context"

	"github.com/Microsoft/hcsshim/internal/hcserror"
	"github.com/Microsoft/hcsshim/internal/ot"
	"go.opentelemetry.io/otel/attribute"
)

// DeactivateLayer will dismount a layer that was mounted via ActivateLayer.
func DeactivateLayer(ctx context.Context, path string) (err error) {
	title := "hcsshim::DeactivateLayer"
	ctx, span := ot.StartSpan(ctx, title) //nolint:ineffassign,staticcheck
	defer span.End()
	defer func() { ot.SetSpanStatus(span, err) }()
	span.SetAttributes(attribute.String("path", path))

	err = deactivateLayer(&stdDriverInfo, path)
	if err != nil {
		return hcserror.New(err, title+"- failed", "")
	}
	return nil
}
