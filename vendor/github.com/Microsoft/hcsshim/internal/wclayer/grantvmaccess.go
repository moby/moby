//go:build windows

package wclayer

import (
	"context"
	"go.opentelemetry.io/otel/attribute"

	"github.com/Microsoft/hcsshim/internal/hcserror"
	"github.com/Microsoft/hcsshim/internal/ot"
)

// GrantVmAccess adds access to a file for a given VM
func GrantVmAccess(ctx context.Context, vmid string, filepath string) (err error) {
	title := "hcsshim::GrantVmAccess"
	ctx, span := ot.StartSpan(ctx, title) //nolint:ineffassign,staticcheck
	defer span.End()
	defer func() { ot.SetSpanStatus(span, err) }()
	span.SetAttributes(
		attribute.String("vm-id", vmid),
		attribute.String("path", filepath))

	err = grantVmAccess(vmid, filepath)
	if err != nil {
		return hcserror.New(err, title, "")
	}
	return nil
}
