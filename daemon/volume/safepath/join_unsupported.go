//go:build !linux && !windows

package safepath

import (
	"context"

	"github.com/moby/moby/v2/errdefs"
)

// Join is not implemented on platforms without a native safepath mount
// strategy. It returns a [errdefs.PlatformNotImplemented] so callers can
// detect the missing implementation via [cerrdefs.IsNotImplemented].
func Join(_ context.Context, _, _ string) (*SafePath, error) {
	return nil, errdefs.PlatformNotImplemented{Feature: "safepath.Join"}
}
