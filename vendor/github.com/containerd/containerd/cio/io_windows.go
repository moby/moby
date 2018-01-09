package cio

import (
	"fmt"
	"io"
	"net"
	"sync"

	winio "github.com/Microsoft/go-winio"
	"github.com/containerd/containerd/log"
	"github.com/pkg/errors"
)

const pipeRoot = `\\.\pipe`

// NewFIFOSetInDir returns a new set of fifos for the task
func NewFIFOSetInDir(_, id string, terminal bool) (*FIFOSet, error) {
	return NewFIFOSet(Config{
		Terminal: terminal,
		Stdin:    fmt.Sprintf(`%s\ctr-%s-stdin`, pipeRoot, id),
		Stdout:   fmt.Sprintf(`%s\ctr-%s-stdout`, pipeRoot, id),
		Stderr:   fmt.Sprintf(`%s\ctr-%s-stderr`, pipeRoot, id),
	}, nil), nil
}

func copyIO(fifos *FIFOSet, ioset *Streams) (*cio, error) {
	var (
		wg  sync.WaitGroup
		set []io.Closer
	)

	if fifos.Stdin != "" {
		l, err := winio.ListenPipe(fifos.Stdin, nil)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to create stdin pipe %s", fifos.Stdin)
		}
		defer func(l net.Listener) {
			if err != nil {
				l.Close()
			}
		}(l)
		set = append(set, l)

		go func() {
			c, err := l.Accept()
			if err != nil {
				log.L.WithError(err).Errorf("failed to accept stdin connection on %s", fifos.Stdin)
				return
			}
			io.Copy(c, ioset.Stdin)
			c.Close()
			l.Close()
		}()
	}

	if fifos.Stdout != "" {
		l, err := winio.ListenPipe(fifos.Stdout, nil)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to create stdin pipe %s", fifos.Stdout)
		}
		defer func(l net.Listener) {
			if err != nil {
				l.Close()
			}
		}(l)
		set = append(set, l)

		wg.Add(1)
		go func() {
			defer wg.Done()
			c, err := l.Accept()
			if err != nil {
				log.L.WithError(err).Errorf("failed to accept stdout connection on %s", fifos.Stdout)
				return
			}
			io.Copy(ioset.Stdout, c)
			c.Close()
			l.Close()
		}()
	}

	if !fifos.Terminal && fifos.Stderr != "" {
		l, err := winio.ListenPipe(fifos.Stderr, nil)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to create stderr pipe %s", fifos.Stderr)
		}
		defer func(l net.Listener) {
			if err != nil {
				l.Close()
			}
		}(l)
		set = append(set, l)

		wg.Add(1)
		go func() {
			defer wg.Done()
			c, err := l.Accept()
			if err != nil {
				log.L.WithError(err).Errorf("failed to accept stderr connection on %s", fifos.Stderr)
				return
			}
			io.Copy(ioset.Stderr, c)
			c.Close()
			l.Close()
		}()
	}

	return &cio{config: fifos.Config, closers: set}, nil
}

// NewDirectIO returns an IO implementation that exposes the IO streams as io.ReadCloser
// and io.WriteCloser.
func NewDirectIO(stdin io.WriteCloser, stdout, stderr io.ReadCloser, terminal bool) *DirectIO {
	return &DirectIO{
		pipes: pipes{
			Stdin:  stdin,
			Stdout: stdout,
			Stderr: stderr,
		},
		cio: cio{
			config: Config{Terminal: terminal},
		},
	}
}
