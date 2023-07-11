//go:build !windows

/*
   Copyright The containerd Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package cio

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"syscall"

	"github.com/containerd/fifo"
)

// NewFIFOSetInDir returns a new FIFOSet with paths in a temporary directory under root
func NewFIFOSetInDir(root, id string, terminal bool) (*FIFOSet, error) {
	if root != "" {
		if err := os.MkdirAll(root, 0700); err != nil {
			return nil, err
		}
	}
	dir, err := os.MkdirTemp(root, "")
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
			p := bufPool.Get().(*[]byte)
			defer bufPool.Put(p)

			io.CopyBuffer(pipes.Stdin, ioset.Stdin, *p)
			pipes.Stdin.Close()
		}()
	}

	var wg = &sync.WaitGroup{}
	if fifos.Stdout != "" {
		wg.Add(1)
		go func() {
			p := bufPool.Get().(*[]byte)
			defer bufPool.Put(p)

			io.CopyBuffer(ioset.Stdout, pipes.Stdout, *p)
			pipes.Stdout.Close()
			wg.Done()
		}()
	}

	if !fifos.Terminal && fifos.Stderr != "" {
		wg.Add(1)
		go func() {
			p := bufPool.Get().(*[]byte)
			defer bufPool.Put(p)

			io.CopyBuffer(ioset.Stderr, pipes.Stderr, *p)
			pipes.Stderr.Close()
			wg.Done()
		}()
	}
	return &cio{
		config:  fifos.Config,
		wg:      wg,
		closers: append(pipes.closers(), fifos),
		cancel: func() {
			cancel()
			for _, c := range pipes.closers() {
				if c != nil {
					c.Close()
				}
			}
		},
	}, nil
}

func openFifos(ctx context.Context, fifos *FIFOSet) (f pipes, retErr error) {
	defer func() {
		if retErr != nil {
			fifos.Close()
		}
	}()

	if fifos.Stdin != "" {
		if f.Stdin, retErr = fifo.OpenFifo(ctx, fifos.Stdin, syscall.O_WRONLY|syscall.O_CREAT|syscall.O_NONBLOCK, 0700); retErr != nil {
			return f, fmt.Errorf("failed to open stdin fifo: %w", retErr)
		}
		defer func() {
			if retErr != nil && f.Stdin != nil {
				f.Stdin.Close()
			}
		}()
	}
	if fifos.Stdout != "" {
		if f.Stdout, retErr = fifo.OpenFifo(ctx, fifos.Stdout, syscall.O_RDONLY|syscall.O_CREAT|syscall.O_NONBLOCK, 0700); retErr != nil {
			return f, fmt.Errorf("failed to open stdout fifo: %w", retErr)
		}
		defer func() {
			if retErr != nil && f.Stdout != nil {
				f.Stdout.Close()
			}
		}()
	}
	if !fifos.Terminal && fifos.Stderr != "" {
		if f.Stderr, retErr = fifo.OpenFifo(ctx, fifos.Stderr, syscall.O_RDONLY|syscall.O_CREAT|syscall.O_NONBLOCK, 0700); retErr != nil {
			return f, fmt.Errorf("failed to open stderr fifo: %w", retErr)
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
