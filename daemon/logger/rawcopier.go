package logger

import (
	"io"
	"sync"

	"github.com/Sirupsen/logrus"
)

// RawCopier can copy logs from specified sources
type RawCopier struct {
	// srcs is map of name -> reader pairs, for example "stdout", "stderr"
	srcs     map[string]io.Reader
	dst      RawLogger
	copyJobs sync.WaitGroup
	closed   chan struct{}
}

// NewRawCopier creates a new RawCopier
func NewRawCopier(srcs map[string]io.Reader, dst RawLogger) *RawCopier {
	return &RawCopier{
		srcs:   srcs,
		dst:    dst,
		closed: make(chan struct{}),
	}
}

// Run starts logs copying
func (c *RawCopier) Run() {
	for src, w := range c.srcs {
		c.copyJobs.Add(1)
		go c.copySrc(src, w)
	}
}

func (c *RawCopier) copySrc(name string, src io.Reader) {
	defer c.copyJobs.Done()
	w, err := c.dst.RawWriter(name)
	if err != nil {
		logrus.Errorf("error while opening RawWriter for %s: %v", name, err)
		return
	}
	for {
		select {
		case <-c.closed:
			if err = w.Close(); err != nil {
				logrus.Errorf("error while closing RawWriter for %s: %v", name, err)
			}
			return
		default:
			// use io.CopyN rather than io.Copy, so that we can catch <-c.closed
			bufsz := int64(64 * 1024)
			if _, err := io.CopyN(w, src, bufsz); err != nil {
				logrus.Errorf("stream copy error: %s: %v", name, err)
				return
			}
		}
	}
}

// Wait waits until all copying is done
func (c *RawCopier) Wait() {
	c.copyJobs.Wait()
}

// Close closes the copier
func (c *RawCopier) Close() {
	select {
	case <-c.closed:
	default:
		close(c.closed)
	}
}
