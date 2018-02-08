// +build !windows,!linux

package shim

import (
	"context"
	"io"
	"sync"
	"syscall"

	"github.com/containerd/console"
	"github.com/containerd/fifo"
)

type unixPlatform struct {
}

func (p *unixPlatform) CopyConsole(ctx context.Context, console console.Console, stdin, stdout, stderr string, wg, cwg *sync.WaitGroup) (console.Console, error) {
	if stdin != "" {
		in, err := fifo.OpenFifo(ctx, stdin, syscall.O_RDONLY, 0)
		if err != nil {
			return nil, err
		}
		cwg.Add(1)
		go func() {
			cwg.Done()
			io.Copy(console, in)
		}()
	}
	outw, err := fifo.OpenFifo(ctx, stdout, syscall.O_WRONLY, 0)
	if err != nil {
		return nil, err
	}
	outr, err := fifo.OpenFifo(ctx, stdout, syscall.O_RDONLY, 0)
	if err != nil {
		return nil, err
	}
	wg.Add(1)
	cwg.Add(1)
	go func() {
		cwg.Done()
		io.Copy(outw, console)
		console.Close()
		outr.Close()
		outw.Close()
		wg.Done()
	}()
	return console, nil
}

func (p *unixPlatform) ShutdownConsole(ctx context.Context, cons console.Console) error {
	return nil
}

func (p *unixPlatform) Close() error {
	return nil
}

func (s *Service) initPlatform() error {
	s.platform = &unixPlatform{}
	return nil
}
