package logger

import (
	"bufio"
	"io"
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
	srcs map[string]io.Reader
	dst  Logger
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
		go c.copySrc(src, w)
	}
}

func (c *Copier) copySrc(name string, src io.Reader) {
	scanner := bufio.NewScanner(src)
	for scanner.Scan() {
		if err := c.dst.Log(&Message{ContainerID: c.cid, Line: scanner.Bytes(), Source: name, Timestamp: time.Now().UTC()}); err != nil {
			logrus.Errorf("Failed to log msg %q for logger %s: %s", scanner.Bytes(), c.dst.Name(), err)
		}
	}
	if err := scanner.Err(); err != nil {
		logrus.Errorf("Error scanning log stream: %s", err)
	}
}
