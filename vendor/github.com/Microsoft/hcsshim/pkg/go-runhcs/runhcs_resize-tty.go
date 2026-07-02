//go:build windows

package runhcs

import (
	"context"
	"strconv"
)

// ResizeTTYOpts is set of options that can be used with the ResizeTTY command.
type ResizeTTYOpts struct {
	// Pid is the process pid (defaults to init pid).
	Pid *int
}

func (opt *ResizeTTYOpts) args() ([]string, error) {
	var out []string
	if opt.Pid != nil {
		out = append(out, "--pid", strconv.Itoa(*opt.Pid))
	}
	return out, nil
}

// ResizeTTY updates the terminal size for a container process.
func (r *Runhcs) ResizeTTY(ctx context.Context, id string, width, height uint16, opts *ResizeTTYOpts) error {
	args := []string{"resize-tty"}
	if opts != nil {
		oargs, err := opts.args()
		if err != nil {
			return err
		}
		args = append(args, oargs...)
	}
	return r.runOrError(r.command(ctx, append(args, id, strconv.FormatUint(uint64(width), 10), strconv.FormatUint(uint64(height), 10))...))
}
