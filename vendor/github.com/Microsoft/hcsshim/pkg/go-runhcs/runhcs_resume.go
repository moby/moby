//go:build windows

package runhcs

import (
	"context"
)

// Resume resumes all processes that have been previously paused.
func (r *Runhcs) Resume(ctx context.Context, id string) error {
	return r.runOrError(r.command(ctx, "resume", id))
}
