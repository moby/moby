//go:build windows

package runhcs

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	irunhcs "github.com/Microsoft/hcsshim/internal/runhcs"
	runc "github.com/containerd/go-runc"
)

// CreateOpts is set of options that can be used with the Create command.
type CreateOpts struct {
	runc.IO
	// PidFile is the path to the file to write the process id to.
	PidFile string
	// ShimLog is the path to the log file or named pipe (e.g. \\.\pipe\ProtectedPrefix\Administrators\runhcs-<container-id>-shim-log) for the launched shim process.
	ShimLog string
	// VMLog is the path to the log file or named pipe (e.g. \\.\pipe\ProtectedPrefix\Administrators\runhcs-<container-id>-vm-log) for the launched VM shim process.
	VMLog string
	// VMConsole is the path to the pipe for the VM's console (e.g. \\.\pipe\debugpipe)
	VMConsole string
}

func (opt *CreateOpts) args() ([]string, error) {
	var out []string
	if opt.PidFile != "" {
		abs, err := filepath.Abs(opt.PidFile)
		if err != nil {
			return nil, err
		}
		out = append(out, "--pid-file", abs)
	}
	if opt.ShimLog != "" {
		if strings.HasPrefix(opt.ShimLog, irunhcs.SafePipePrefix) {
			out = append(out, "--shim-log", opt.ShimLog)
		} else {
			abs, err := filepath.Abs(opt.ShimLog)
			if err != nil {
				return nil, err
			}
			out = append(out, "--shim-log", abs)
		}
	}
	if opt.VMLog != "" {
		if strings.HasPrefix(opt.VMLog, irunhcs.SafePipePrefix) {
			out = append(out, "--vm-log", opt.VMLog)
		} else {
			abs, err := filepath.Abs(opt.VMLog)
			if err != nil {
				return nil, err
			}
			out = append(out, "--vm-log", abs)
		}
	}
	if opt.VMConsole != "" {
		out = append(out, "--vm-console", opt.VMConsole)
	}
	return out, nil
}

// Create creates a new container and returns its pid if it was created
// successfully.
func (r *Runhcs) Create(ctx context.Context, id, bundle string, opts *CreateOpts) error {
	args := []string{"create", "--bundle", bundle}
	if opts != nil {
		oargs, err := opts.args()
		if err != nil {
			return err
		}
		args = append(args, oargs...)
	}
	cmd := r.command(ctx, append(args, id)...)
	if opts != nil && opts.IO != nil {
		opts.Set(cmd)
	}
	if cmd.Stdout == nil && cmd.Stderr == nil {
		data, err := cmdOutput(cmd, true)
		if err != nil {
			return fmt.Errorf("%s: %s", err, data) //nolint:errorlint // legacy code
		}
		return nil
	}
	ec, err := runc.Monitor.Start(cmd)
	if err != nil {
		return err
	}
	if opts != nil && opts.IO != nil {
		if c, ok := opts.IO.(runc.StartCloser); ok {
			if err := c.CloseAfterStart(); err != nil {
				return err
			}
		}
	}
	status, err := runc.Monitor.Wait(cmd, ec)
	if err == nil && status != 0 {
		err = fmt.Errorf("%s did not terminate successfully", cmd.Args[0])
	}
	return err
}
