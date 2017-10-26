package libcontainerd

import (
	"context"
	"io"
	"net"
	"sync"

	winio "github.com/Microsoft/go-winio"
	"github.com/containerd/containerd"
	"github.com/pkg/errors"
)

type winpipe struct {
	sync.Mutex

	ctx      context.Context
	listener net.Listener
	readyCh  chan struct{}
	readyErr error

	client net.Conn
}

func newWinpipe(ctx context.Context, pipe string) (*winpipe, error) {
	l, err := winio.ListenPipe(pipe, nil)
	if err != nil {
		return nil, errors.Wrapf(err, "%q pipe creation failed", pipe)
	}
	wp := &winpipe{
		ctx:      ctx,
		listener: l,
		readyCh:  make(chan struct{}),
	}
	go func() {
		go func() {
			defer close(wp.readyCh)
			defer wp.listener.Close()
			c, err := wp.listener.Accept()
			if err != nil {
				wp.Lock()
				if wp.readyErr == nil {
					wp.readyErr = err
				}
				wp.Unlock()
				return
			}
			wp.client = c
		}()

		select {
		case <-wp.readyCh:
		case <-ctx.Done():
			wp.Lock()
			if wp.readyErr == nil {
				wp.listener.Close()
				wp.readyErr = ctx.Err()
			}
			wp.Unlock()
		}
	}()

	return wp, nil
}

func (wp *winpipe) Read(b []byte) (int, error) {
	select {
	case <-wp.ctx.Done():
		return 0, wp.ctx.Err()
	case <-wp.readyCh:
		return wp.client.Read(b)
	}
}

func (wp *winpipe) Write(b []byte) (int, error) {
	select {
	case <-wp.ctx.Done():
		return 0, wp.ctx.Err()
	case <-wp.readyCh:
		return wp.client.Write(b)
	}
}

func (wp *winpipe) Close() error {
	select {
	case <-wp.readyCh:
		return wp.client.Close()
	default:
		return nil
	}
}

func newIOPipe(fifos *containerd.FIFOSet) (*IOPipe, error) {
	var (
		err         error
		ctx, cancel = context.WithCancel(context.Background())
		p           io.ReadWriteCloser
		iop         = &IOPipe{
			Terminal: fifos.Terminal,
			cancel:   cancel,
			config: containerd.IOConfig{
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
		if p, err = newWinpipe(ctx, fifos.In); err != nil {
			return nil, err
		}
		iop.Stdin = p
	}

	if fifos.Out != "" {
		if p, err = newWinpipe(ctx, fifos.Out); err != nil {
			return nil, err
		}
		iop.Stdout = p
	}

	if fifos.Err != "" {
		if p, err = newWinpipe(ctx, fifos.Err); err != nil {
			return nil, err
		}
		iop.Stderr = p
	}

	return iop, nil
}
