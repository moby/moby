package logger

import (
	"bufio"
	"bytes"
	"io"
	"sync"
	"time"

	"github.com/Sirupsen/logrus"
)

// Copier can copy logs from specified sources to Logger and attach Timestamp.
// Writes are concurrent, so you need implement some sync in your logger
type Copier struct {
	// srcs is map of name -> reader pairs, for example "stdout", "stderr"
	srcs     map[string]io.Reader
	dst      Logger
	copyJobs sync.WaitGroup
	closed   chan struct{}
}

// NewCopier creates a new Copier
func NewCopier(srcs map[string]io.Reader, dst Logger) *Copier {
	return &Copier{
		srcs:   srcs,
		dst:    dst,
		closed: make(chan struct{}),
	}
}

// Run starts logs copying
func (c *Copier) Run() {
	for src, w := range c.srcs {
		c.copyJobs.Add(1)
		go c.copySrc(src, w)
	}
}

func (c *Copier) copySrc(name string, src io.Reader) {
	defer c.copyJobs.Done()
	reader := bufio.NewReader(src)

	for {
		select {
		case <-c.closed:
			return
		default:
			line, err := reader.ReadBytes('\n')
			line = bytes.TrimSuffix(line, []byte{'\n'})

			// ReadBytes can return full or partial output even when it failed.
			// e.g. it can return a full entry and EOF.
			if err == nil || len(line) > 0 {
				if logErr := c.dst.Log(&Message{Line: line, Source: name, Timestamp: time.Now().UTC()}); logErr != nil {
					logrus.Errorf("Failed to log msg %q for logger %s: %s", line, c.dst.Name(), logErr)
				}
			}

			if err != nil {
				if err != io.EOF {
					logrus.Errorf("Error scanning log stream: %s", err)
				}
				return
			}
		}
	}
}

// Wait waits until all copying is done
func (c *Copier) Wait() {
	c.copyJobs.Wait()
}

// Close closes the copier
func (c *Copier) Close() {
	select {
	case <-c.closed:
	default:
		close(c.closed)
	}
}
