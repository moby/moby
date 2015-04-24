package logger

import (
	"bytes"
	"io"
	"sync"
	"time"

	"github.com/Sirupsen/logrus"
)

// Copier can copy logs from specified sources to Logger and attach
// ContainerID and Timestamp.
// Writes are concurrent, so you need implement some sync in your logger
type Copier struct {
	// cid is container id for which we copying logs
	cid string
	// srcs is map of name -> reader pairs, for example "stdout", "stderr"
	srcs     map[string]io.Reader
	dst      Logger
	copyJobs sync.WaitGroup
}

// NewCopier creates new Copier
func NewCopier(cid string, srcs map[string]io.Reader, dst Logger) (*Copier, error) {
	return &Copier{
		cid:  cid,
		srcs: srcs,
		dst:  dst,
	}, nil
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

	buf := new(bytes.Buffer)
	if _, err := buf.ReadFrom(src); err != nil {
		logrus.Errorf("Error reading log stream with logger %s: %s", c.dst.Name(), err)
	}

	bytes := buf.Bytes()
	if len(bytes) == 0 {
		return
	}

	if err := c.dst.Log(&Message{ContainerID: c.cid, Line: bytes, Source: name, Timestamp: time.Now().UTC()}); err != nil {
		logrus.Errorf("Failed to log msg %q for logger %s: %s", bytes, c.dst.Name(), err)
	}
}

// Wait waits until all copying is done
func (c *Copier) Wait() {
	c.copyJobs.Wait()
}
