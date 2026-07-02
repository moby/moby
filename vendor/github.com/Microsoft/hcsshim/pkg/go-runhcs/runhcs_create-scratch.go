//go:build windows

package runhcs

import (
	"context"
	"errors"
	"path/filepath"
	"strconv"
)

// CreateScratch creates a scratch vhdx at 'destpath' that is ext4 formatted.
func (r *Runhcs) CreateScratch(ctx context.Context, destpath string) error {
	return r.CreateScratchWithOpts(ctx, destpath, nil)
}

// CreateScratchOpts is the set of options that can be used with the
// `CreateScratchWithOpts` command.
type CreateScratchOpts struct {
	// SizeGB is the size in GB of the scratch file to create.
	SizeGB int
	// CacheFile is the path to an existing `scratch.vhx` to copy. If
	// `CacheFile` does not exit the scratch will be created.
	CacheFile string
	// UseVirtualMemory indicates whether the UVM used to create the
	// scratch should be backed with virtual memory or not.
	UseVirtualMemory bool
}

func (opt *CreateScratchOpts) args() ([]string, error) {
	var out []string
	if opt.SizeGB < 0 {
		return nil, errors.New("sizeGB must be >= 0")
	} else if opt.SizeGB > 0 {
		out = append(out, "--sizeGB", strconv.Itoa(opt.SizeGB))
	}
	if opt.CacheFile != "" {
		abs, err := filepath.Abs(opt.CacheFile)
		if err != nil {
			return nil, err
		}
		out = append(out, "--cache-path", abs)
	}
	if opt.UseVirtualMemory {
		out = append(out, "--use-virtual-memory")
	}
	return out, nil
}

// CreateScratchWithOpts creates a scratch vhdx at 'destpath' that is ext4
// formatted based on `opts`.
func (r *Runhcs) CreateScratchWithOpts(ctx context.Context, destpath string, opts *CreateScratchOpts) error {
	args := []string{"create-scratch", "--destpath", destpath}
	if opts != nil {
		oargs, err := opts.args()
		if err != nil {
			return err
		}
		args = append(args, oargs...)
	}
	return r.runOrError(r.command(ctx, args...))
}
