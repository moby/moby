//go:build windows

package computestorage

import (
	"context"
	"encoding/json"
	"go.opentelemetry.io/otel/attribute"

	"github.com/Microsoft/hcsshim/internal/ot"
	"github.com/pkg/errors"
)

// InitializeWritableLayer initializes a writable layer for a container.
//
// `layerPath` is a path to a directory the layer is mounted. If the
// path does not end in a `\` the platform will append it automatically.
//
// `layerData` is the parent read-only layer data.
func InitializeWritableLayer(ctx context.Context, layerPath string, layerData LayerData) (err error) {
	title := "hcsshim::InitializeWritableLayer"
	ctx, span := ot.StartSpan(ctx, title) //nolint:ineffassign,staticcheck
	defer span.End()
	defer func() { ot.SetSpanStatus(span, err) }()
	span.SetAttributes(
		attribute.String("layerPath", layerPath),
	)

	bytes, err := json.Marshal(layerData)
	if err != nil {
		return err
	}

	// Options are not used in the platform as of RS5
	err = hcsInitializeWritableLayer(layerPath, string(bytes), "")
	if err != nil {
		return errors.Wrap(err, "failed to intitialize container layer")
	}
	return nil
}
