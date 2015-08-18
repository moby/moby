package logger

import (
	"bufio"
	"io"
	"sync"
	"time"

	"github.com/Sirupsen/logrus"
)

// MaxBytesPerLine is the max bytes per line of containers' log
const MaxBytesPerLine = 4096

// Copier can copy logs from specified sources to Logger and attach
// ContainerID and Timestamp.
// Writes are concurrent, so you need implement some sync in your logger
type Copier struct {
	// cid is the container id for which we are copying logs
	cid string
	// srcs is map of name -> reader pairs, for example "stdout", "stderr"
	srcs     map[string]io.Reader
	dst      Logger
	copyJobs sync.WaitGroup
}

// NewCopier creates a new Copier
func NewCopier(cid string, srcs map[string]io.Reader, dst Logger) *Copier {
	return &Copier{
		cid:  cid,
		srcs: srcs,
		dst:  dst,
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
	reader := bufio.NewReaderSize(src, MaxBytesPerLine)

	for {
		// ReadLine tries to return a single line, not including the end-of-line bytes.
		// If the line was too long for the buffer then isPrefix is set and the
		// beginning of the line is returned. The rest of the line will be returned
		// from future calls.
		line, _, err := reader.ReadLine()

		if err == nil {
			if logErr := c.dst.Log(&Message{ContainerID: c.cid, Line: line, Source: name, Timestamp: time.Now().UTC()}); logErr != nil {
				logrus.Errorf("Failed to log msg %q for logger %s: %s", line, c.dst.Name(), logErr)
			}
		} else {
			if err != io.EOF {
				logrus.Errorf("Error scanning log stream: %s", err)
			}
			return
		}

	}
}

// Wait waits until all copying is done
func (c *Copier) Wait() {
	c.copyJobs.Wait()
}
