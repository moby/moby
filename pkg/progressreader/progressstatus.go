package progressreader

import (
	"bytes"
	"io"
	"sync"

	"github.com/docker/docker/vendor/src/github.com/Sirupsen/logrus"
)

type ProgressStatus struct {
	sync.Mutex
	c         chan struct{}
	observers []io.Writer
	history   bytes.Buffer
}

func NewProgressStatus() *ProgressStatus {
	return &ProgressStatus{
		c:         make(chan struct{}),
		observers: []io.Writer{},
	}
}

func (ps *ProgressStatus) Write(p []byte) (n int, err error) {
	ps.Lock()
	defer ps.Unlock()
	ps.history.Write(p)
	for _, w := range ps.observers {
		// copy paste from MultiWriter, replaced return with continue
		n, err = w.Write(p)
		if err != nil {
			continue
		}
		if n != len(p) {
			err = io.ErrShortWrite
			continue
		}
	}
	return len(p), nil
}

func (ps *ProgressStatus) AddObserver(w io.Writer) {
	ps.Lock()
	defer ps.Unlock()
	w.Write(ps.history.Bytes())
	ps.observers = append(ps.observers, w)
}

func (ps *ProgressStatus) Done() {
	ps.Lock()
	close(ps.c)
	ps.history.Reset()
	ps.Unlock()
}

func (ps *ProgressStatus) Wait(w io.Writer, msg []byte) error {
	ps.Lock()
	channel := ps.c
	ps.Unlock()

	if channel == nil {
		// defensive
		logrus.Debugf("Channel is nil ")
	}
	if w != nil {
		w.Write(msg)
		ps.AddObserver(w)
	}
	<-channel
	return nil
}
