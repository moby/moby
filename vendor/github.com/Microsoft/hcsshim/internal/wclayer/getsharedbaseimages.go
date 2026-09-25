//go:build windows

package wclayer

import (
	"context"

	"github.com/Microsoft/hcsshim/internal/hcserror"
	"github.com/Microsoft/hcsshim/internal/interop"
	"github.com/Microsoft/hcsshim/internal/ot"
	"go.opentelemetry.io/otel/attribute"
)

// GetSharedBaseImages will enumerate the images stored in the common central
// image store and return descriptive info about those images for the purpose
// of registering them with the graphdriver, graph, and tagstore.
func GetSharedBaseImages(ctx context.Context) (_ string, err error) {
	title := "hcsshim::GetSharedBaseImages"
	ctx, span := ot.StartSpan(ctx, title) //nolint:ineffassign,staticcheck
	defer span.End()
	defer func() { ot.SetSpanStatus(span, err) }()

	var buffer *uint16
	err = getBaseImages(&buffer)
	if err != nil {
		return "", hcserror.New(err, title, "")
	}
	imageData := interop.ConvertAndFreeCoTaskMemString(buffer)
	span.SetAttributes(attribute.String("imageData", imageData))
	return imageData, nil
}
