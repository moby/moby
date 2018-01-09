package cio

import (
	"context"
	"fmt"
	"io"
	"os"
	"sync"
)

// Config holds the IO configurations.
type Config struct {
	// Terminal is true if one has been allocated
	Terminal bool
	// Stdin path
	Stdin string
	// Stdout path
	Stdout string
	// Stderr path
	Stderr string
}

// IO holds the io information for a task or process
type IO interface {
	// Config returns the IO configuration.
	Config() Config
	// Cancel aborts all current io operations.
	Cancel()
	// Wait blocks until all io copy operations have completed.
	Wait()
	// Close cleans up all open io resources. Cancel() is always called before
	// Close()
	Close() error
}

// Creator creates new IO sets for a task
type Creator func(id string) (IO, error)

// Attach allows callers to reattach to running tasks
//
// There should only be one reader for a task's IO set
// because fifo's can only be read from one reader or the output
// will be sent only to the first reads
type Attach func(*FIFOSet) (IO, error)

// FIFOSet is a set of file paths to FIFOs for a task's standard IO streams
type FIFOSet struct {
	Config
	close func() error
}

// Close the FIFOSet
func (f *FIFOSet) Close() error {
	if f.close != nil {
		return f.close()
	}
	return nil
}

// NewFIFOSet returns a new FIFOSet from a Config and a close function
func NewFIFOSet(config Config, close func() error) *FIFOSet {
	return &FIFOSet{Config: config, close: close}
}

// Streams used to configure a Creator or Attach
type Streams struct {
	Stdin    io.Reader
	Stdout   io.Writer
	Stderr   io.Writer
	Terminal bool
}

// Opt customize options for creating a Creator or Attach
type Opt func(*Streams)

// WithStdio sets stream options to the standard input/output streams
func WithStdio(opt *Streams) {
	WithStreams(os.Stdin, os.Stdout, os.Stderr)(opt)
}

// WithTerminal sets the terminal option
func WithTerminal(opt *Streams) {
	opt.Terminal = true
}

// WithStreams sets the stream options to the specified Reader and Writers
func WithStreams(stdin io.Reader, stdout, stderr io.Writer) Opt {
	return func(opt *Streams) {
		opt.Stdin = stdin
		opt.Stdout = stdout
		opt.Stderr = stderr
	}
}

// NewCreator returns an IO creator from the options
func NewCreator(opts ...Opt) Creator {
	streams := &Streams{}
	for _, opt := range opts {
		opt(streams)
	}
	return func(id string) (IO, error) {
		// TODO: accept root as a param
		root := "/run/containerd/fifo"
		fifos, err := NewFIFOSetInDir(root, id, streams.Terminal)
		if err != nil {
			return nil, err
		}
		return copyIO(fifos, streams)
	}
}

// NewAttach attaches the existing io for a task to the provided io.Reader/Writers
func NewAttach(opts ...Opt) Attach {
	streams := &Streams{}
	for _, opt := range opts {
		opt(streams)
	}
	return func(fifos *FIFOSet) (IO, error) {
		if fifos == nil {
			return nil, fmt.Errorf("cannot attach, missing fifos")
		}
		return copyIO(fifos, streams)
	}
}

// NullIO redirects the container's IO into /dev/null
func NullIO(_ string) (IO, error) {
	return &cio{}, nil
}

// cio is a basic container IO implementation.
type cio struct {
	config  Config
	wg      *sync.WaitGroup
	closers []io.Closer
	cancel  context.CancelFunc
}

func (c *cio) Config() Config {
	return c.config
}

func (c *cio) Wait() {
	if c.wg != nil {
		c.wg.Wait()
	}
}

func (c *cio) Close() error {
	var lastErr error
	for _, closer := range c.closers {
		if closer == nil {
			continue
		}
		if err := closer.Close(); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

func (c *cio) Cancel() {
	if c.cancel != nil {
		c.cancel()
	}
}

type pipes struct {
	Stdin  io.WriteCloser
	Stdout io.ReadCloser
	Stderr io.ReadCloser
}

// DirectIO allows task IO to be handled externally by the caller
type DirectIO struct {
	pipes
	cio
}

var _ IO = &DirectIO{}
