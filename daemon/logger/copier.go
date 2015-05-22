package logger

import (
	"bufio"
	"io"
	"sync"
	"time"

	"github.com/Sirupsen/logrus"
)

const (
	// The size of the buffer used to read one line of log.
	MaxLogLineLength = 1024 * 1024
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

	reader := bufio.NewReaderSize(src, MaxLogLineLength)
	line, isPrefix, err := reader.ReadLine()
	for err != io.EOF {
		if err != nil {
			logrus.Errorf("Failed to read log line for container %s: %s", c.cid, err)
			continue
		}
		if isPrefix {
			logrus.Warnf("Log line exceeds buffer size for container %s", c.cid)
		}

		err := c.dst.Log(&Message{ContainerID: c.cid, Line: line, Source: name, Timestamp: time.Now().UTC()})
		if err != nil {
			logrus.Errorf("Failed to log msg for container %s:", c.cid, err)
		}

		line, isPrefix, err = reader.ReadLine()
	}
}

// Wait waits until all copying is done
func (c *Copier) Wait() {
	c.copyJobs.Wait()
}
