//go:build windows

package computestorage

import (
	"context"
	"go.opentelemetry.io/otel/attribute"

	"github.com/Microsoft/hcsshim/internal/ot"
	"github.com/pkg/errors"
)

// DestroyLayer deletes a container layer.
//
// `layerPath` is a path to a directory containing the layer to export.
func DestroyLayer(ctx context.Context, layerPath string) (err error) {
	title := "hcsshim::DestroyLayer"
	ctx, span := ot.StartSpan(ctx, title) //nolint:ineffassign,staticcheck
	defer span.End()
	defer func() { ot.SetSpanStatus(span, err) }()
	span.SetAttributes(attribute.String("layerPath", layerPath))

	err = hcsDestroyLayer(layerPath)
	if err != nil {
		return errors.Wrap(err, "failed to destroy layer")
	}
	return nil
}
