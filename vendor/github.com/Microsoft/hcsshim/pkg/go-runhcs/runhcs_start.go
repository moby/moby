//go:build windows

package runhcs

import (
	"context"
)

// Start will start an already created container.
func (r *Runhcs) Start(ctx context.Context, id string) error {
	return r.runOrError(r.command(ctx, "start", id))
}
