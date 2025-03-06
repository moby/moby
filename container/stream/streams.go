package stream // import "github.com/docker/docker/container/stream"

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"sync/atomic"

	"github.com/containerd/containerd/v2/pkg/cio"
	"github.com/containerd/log"
	"github.com/docker/docker/container/stream/bytespipe"
	"github.com/docker/docker/pkg/pools"
)

// Config holds information about I/O streams managed together.
//
// config.StdinPipe returns a WriteCloser which can be used to feed data
// to the standard input of the streamConfig's active process.
// config.StdoutPipe and streamConfig.StderrPipe each return a ReadCloser
// which can be used to retrieve the standard output (and error) generated
// by the container's active process. The output (and error) are actually
// copied and delivered to all StdoutPipe and StderrPipe consumers, using
// a kind of "broadcaster".
type Config struct {
	wg        sync.WaitGroup
	stdout    *unbuffered
	stderr    *unbuffered
	stdin     io.ReadCloser
	stdinPipe io.WriteCloser
	dio       *cio.DirectIO
	// closed is set to true when CloseStreams is called
	closed atomic.Bool
}

// NewConfig creates a stream config and initializes
// the standard err and standard out to new unbuffered broadcasters.
func NewConfig() *Config {
	return &Config{
		stderr: new(unbuffered),
		stdout: new(unbuffered),
	}
}

// Stdout returns the standard output in the configuration.
func (c *Config) Stdout() io.Writer {
	return c.stdout
}

// Stderr returns the standard error in the configuration.
func (c *Config) Stderr() io.Writer {
	return c.stderr
}

// Stdin returns the standard input in the configuration.
func (c *Config) Stdin() io.ReadCloser {
	return c.stdin
}

// StdinPipe returns an input writer pipe as an io.WriteCloser.
func (c *Config) StdinPipe() io.WriteCloser {
	return c.stdinPipe
}

// StdoutPipe creates a new io.ReadCloser with an empty bytes pipe.
// It adds this new out pipe to the Stdout broadcaster.
// This will block stdout if unconsumed.
func (c *Config) StdoutPipe() io.ReadCloser {
	bytesPipe := bytespipe.New()
	c.stdout.Add(bytesPipe)
	return bytesPipe
}

// StderrPipe creates a new io.ReadCloser with an empty bytes pipe.
// It adds this new err pipe to the Stderr broadcaster.
// This will block stderr if unconsumed.
func (c *Config) StderrPipe() io.ReadCloser {
	bytesPipe := bytespipe.New()
	c.stderr.Add(bytesPipe)
	return bytesPipe
}

// NewInputPipes creates new pipes for both standard inputs, Stdin and StdinPipe.
func (c *Config) NewInputPipes() {
	c.stdin, c.stdinPipe = io.Pipe()
}

// NewNopInputPipe creates a new input pipe that will silently drop all messages in the input.
func (c *Config) NewNopInputPipe() {
	c.stdinPipe = &nopWriteCloser{io.Discard}
}

type nopWriteCloser struct {
	io.Writer
}

func (w *nopWriteCloser) Close() error { return nil }

// CloseStreams ensures that the configured streams are properly closed.
func (c *Config) CloseStreams() error {
	var errs error

	c.closed.Store(true)

	if c.stdin != nil {
		if err := c.stdin.Close(); err != nil {
			errs = errors.Join(errs, fmt.Errorf("error close stdin: %w", err))
		}
	}

	if err := c.stdout.Clean(); err != nil {
		errs = errors.Join(errs, fmt.Errorf("error close stdout: %w", err))
	}

	if err := c.stderr.Clean(); err != nil {
		errs = errors.Join(errs, fmt.Errorf("error close stderr: %w", err))
	}

	return errs
}

// CopyToPipe connects streamconfig with a libcontainerd.IOPipe
func (c *Config) CopyToPipe(iop *cio.DirectIO) {
	ctx := context.TODO()

	c.dio = iop
	copyFunc := func(name string, w io.Writer, r io.ReadCloser) {
		c.wg.Add(1)
		go func() {
			defer c.wg.Done()
			if _, err := pools.Copy(w, r); err != nil {
				if c.closed.Load() {
					return
				}
				log.G(ctx).WithFields(log.Fields{"stream": name, "error": err}).Error("copy stream failed")
			}
			if err := r.Close(); err != nil && !c.closed.Load() {
				log.G(ctx).WithFields(log.Fields{"stream": name, "error": err}).Warn("close stream failed")
			}
		}()
	}

	if iop.Stdout != nil {
		copyFunc("stdout", c.Stdout(), iop.Stdout)
	}
	if iop.Stderr != nil {
		copyFunc("stderr", c.Stderr(), iop.Stderr)
	}

	if stdin := c.Stdin(); stdin != nil {
		if iop.Stdin != nil {
			go func() {
				_, err := pools.Copy(iop.Stdin, stdin)
				if err != nil {
					if c.closed.Load() {
						return
					}
					log.G(ctx).WithFields(log.Fields{"stream": "stdin", "error": err}).Error("copy stream failed")
				}
				if err := iop.Stdin.Close(); err != nil && !c.closed.Load() {
					log.G(ctx).WithFields(log.Fields{"stream": "stdin", "error": err}).Warn("close stream failed")
				}
			}()
		}
	}
}

// Wait for the stream to close
// Wait supports timeouts via the context to unblock and forcefully
// close the io streams
func (c *Config) Wait(ctx context.Context) {
	done := make(chan struct{}, 1)
	go func() {
		c.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-ctx.Done():
		if c.dio != nil {
			c.dio.Cancel()
			c.dio.Wait()
			c.dio.Close()
		}
	}
}
