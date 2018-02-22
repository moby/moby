// +build !windows

package cio

import (
	"context"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"syscall"

	"github.com/containerd/fifo"
	"github.com/pkg/errors"
)

// NewFIFOSetInDir returns a new FIFOSet with paths in a temporary directory under root
func NewFIFOSetInDir(root, id string, terminal bool) (*FIFOSet, error) {
	if root != "" {
		if err := os.MkdirAll(root, 0700); err != nil {
			return nil, err
		}
	}
	dir, err := ioutil.TempDir(root, "")
	if err != nil {
		return nil, err
	}
	closer := func() error {
		return os.RemoveAll(dir)
	}
	return NewFIFOSet(Config{
		Stdin:    filepath.Join(dir, id+"-stdin"),
		Stdout:   filepath.Join(dir, id+"-stdout"),
		Stderr:   filepath.Join(dir, id+"-stderr"),
		Terminal: terminal,
	}, closer), nil
}

func copyIO(fifos *FIFOSet, ioset *Streams) (*cio, error) {
	var ctx, cancel = context.WithCancel(context.Background())
	pipes, err := openFifos(ctx, fifos)
	if err != nil {
		cancel()
		return nil, err
	}

	if fifos.Stdin != "" {
		go func() {
			io.Copy(pipes.Stdin, ioset.Stdin)
			pipes.Stdin.Close()
		}()
	}

	var wg = &sync.WaitGroup{}
	wg.Add(1)
	go func() {
		io.Copy(ioset.Stdout, pipes.Stdout)
		pipes.Stdout.Close()
		wg.Done()
	}()

	if !fifos.Terminal {
		wg.Add(1)
		go func() {
			io.Copy(ioset.Stderr, pipes.Stderr)
			pipes.Stderr.Close()
			wg.Done()
		}()
	}
	return &cio{
		config:  fifos.Config,
		wg:      wg,
		closers: append(pipes.closers(), fifos),
		cancel:  cancel,
	}, nil
}

func openFifos(ctx context.Context, fifos *FIFOSet) (pipes, error) {
	var err error
	defer func() {
		if err != nil {
			fifos.Close()
		}
	}()

	var f pipes
	if fifos.Stdin != "" {
		if f.Stdin, err = fifo.OpenFifo(ctx, fifos.Stdin, syscall.O_WRONLY|syscall.O_CREAT|syscall.O_NONBLOCK, 0700); err != nil {
			return f, errors.Wrapf(err, "failed to open stdin fifo")
		}
	}
	if fifos.Stdout != "" {
		if f.Stdout, err = fifo.OpenFifo(ctx, fifos.Stdout, syscall.O_RDONLY|syscall.O_CREAT|syscall.O_NONBLOCK, 0700); err != nil {
			f.Stdin.Close()
			return f, errors.Wrapf(err, "failed to open stdout fifo")
		}
	}
	if fifos.Stderr != "" {
		if f.Stderr, err = fifo.OpenFifo(ctx, fifos.Stderr, syscall.O_RDONLY|syscall.O_CREAT|syscall.O_NONBLOCK, 0700); err != nil {
			f.Stdin.Close()
			f.Stdout.Close()
			return f, errors.Wrapf(err, "failed to open stderr fifo")
		}
	}
	return f, nil
}

// NewDirectIO returns an IO implementation that exposes the IO streams as io.ReadCloser
// and io.WriteCloser.
func NewDirectIO(ctx context.Context, fifos *FIFOSet) (*DirectIO, error) {
	ctx, cancel := context.WithCancel(ctx)
	pipes, err := openFifos(ctx, fifos)
	return &DirectIO{
		pipes: pipes,
		cio: cio{
			config:  fifos.Config,
			closers: append(pipes.closers(), fifos),
			cancel:  cancel,
		},
	}, err
}

func (p *pipes) closers() []io.Closer {
	return []io.Closer{p.Stdin, p.Stdout, p.Stderr}
}
