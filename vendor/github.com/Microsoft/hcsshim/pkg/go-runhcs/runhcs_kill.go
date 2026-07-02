//go:build windows

package runhcs

import (
	"context"
)

// Kill sends the specified signal (default: SIGTERM) to the container's init
// process.
func (r *Runhcs) Kill(ctx context.Context, id, signal string) error {
	return r.runOrError(r.command(ctx, "kill", id, signal))
}
