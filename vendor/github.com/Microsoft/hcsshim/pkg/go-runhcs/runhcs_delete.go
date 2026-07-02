//go:build windows

package runhcs

import (
	"context"
)

// DeleteOpts is set of options that can be used with the Delete command.
type DeleteOpts struct {
	// Force forcibly deletes the container if it is still running (uses SIGKILL).
	Force bool
}

func (opt *DeleteOpts) args() ([]string, error) {
	var out []string
	if opt.Force {
		out = append(out, "--force")
	}
	return out, nil
}

// Delete any resources held by the container often used with detached
// containers.
func (r *Runhcs) Delete(ctx context.Context, id string, opts *DeleteOpts) error {
	args := []string{"delete"}
	if opts != nil {
		oargs, err := opts.args()
		if err != nil {
			return err
		}
		args = append(args, oargs...)
	}
	return r.runOrError(r.command(ctx, append(args, id)...))
}
