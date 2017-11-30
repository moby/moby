// +build !windows

package libcontainerd

import (
	"context"
	"io"
	"syscall"

	"github.com/containerd/containerd/cio"
	"github.com/containerd/fifo"
	"github.com/pkg/errors"
)

func newIOPipe(fifos *cio.FIFOSet) (*IOPipe, error) {
	var (
		err         error
		ctx, cancel = context.WithCancel(context.Background())
		f           io.ReadWriteCloser
		iop         = &IOPipe{
			Terminal: fifos.Terminal,
			cancel:   cancel,
			config: cio.Config{
				Terminal: fifos.Terminal,
				Stdin:    fifos.In,
				Stdout:   fifos.Out,
				Stderr:   fifos.Err,
			},
		}
	)
	defer func() {
		if err != nil {
			cancel()
			iop.Close()
		}
	}()

	if fifos.In != "" {
		if f, err = fifo.OpenFifo(ctx, fifos.In, syscall.O_WRONLY|syscall.O_CREAT|syscall.O_NONBLOCK, 0700); err != nil {
			return nil, errors.WithStack(err)
		}
		iop.Stdin = f
	}

	if fifos.Out != "" {
		if f, err = fifo.OpenFifo(ctx, fifos.Out, syscall.O_RDONLY|syscall.O_CREAT|syscall.O_NONBLOCK, 0700); err != nil {
			return nil, errors.WithStack(err)
		}
		iop.Stdout = f
	}

	if fifos.Err != "" {
		if f, err = fifo.OpenFifo(ctx, fifos.Err, syscall.O_RDONLY|syscall.O_CREAT|syscall.O_NONBLOCK, 0700); err != nil {
			return nil, errors.WithStack(err)
		}
		iop.Stderr = f
	}

	return iop, nil
}
