package loggerutils

import (
	"os"
	"strconv"
	"sync"

	"github.com/docker/docker/daemon/logger"
	"github.com/docker/docker/pkg/pubsub"
	"github.com/pkg/errors"
)

// RotateFileWriter is Logger implementation for default Docker logging.
type RotateFileWriter struct {
	f            *os.File // store for closing
	closed       bool
	mu           sync.Mutex
	capacity     int64 //maximum size of each file
	currentSize  int64 // current size of the latest file
	maxFiles     int   //maximum number of files
	notifyRotate *pubsub.Publisher
	marshal      logger.MarshalFunc
}

//NewRotateFileWriter creates new RotateFileWriter
func NewRotateFileWriter(logPath string, capacity int64, maxFiles int, marshaller logger.MarshalFunc) (*RotateFileWriter, error) {
	log, err := os.OpenFile(logPath, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0640)
	if err != nil {
		return nil, err
	}

	size, err := log.Seek(0, os.SEEK_END)
	if err != nil {
		return nil, err
	}

	return &RotateFileWriter{
		f:            log,
		capacity:     capacity,
		currentSize:  size,
		maxFiles:     maxFiles,
		notifyRotate: pubsub.NewPublisher(0, 1),
		marshal:      marshaller,
	}, nil
}

// WriteLogEntry writes the provided log message to the current log file.
// This may trigger a rotation event if the max file/capacity limits are hit.
func (w *RotateFileWriter) WriteLogEntry(msg *logger.Message) error {
	b, err := w.marshal(msg)
	if err != nil {
		return errors.Wrap(err, "error marshalling log message")
	}

	logger.PutMessage(msg)

	w.mu.Lock()
	if w.closed {
		w.mu.Unlock()
		return errors.New("cannot write because the output file was closed")
	}

	if err := w.checkCapacityAndRotate(); err != nil {
		w.mu.Unlock()
		return err
	}

	n, err := w.f.Write(b)
	if err == nil {
		w.currentSize += int64(n)
	}
	w.mu.Unlock()
	return err
}

func (w *RotateFileWriter) checkCapacityAndRotate() error {
	if w.capacity == -1 {
		return nil
	}

	if w.currentSize >= w.capacity {
		name := w.f.Name()
		if err := w.f.Close(); err != nil {
			return errors.Wrap(err, "error closing file")
		}
		if err := rotate(name, w.maxFiles); err != nil {
			return err
		}
		file, err := os.OpenFile(name, os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0640)
		if err != nil {
			return err
		}
		w.f = file
		w.currentSize = 0
		w.notifyRotate.Publish(struct{}{})
	}

	return nil
}

func rotate(name string, maxFiles int) error {
	if maxFiles < 2 {
		return nil
	}
	for i := maxFiles - 1; i > 1; i-- {
		toPath := name + "." + strconv.Itoa(i)
		fromPath := name + "." + strconv.Itoa(i-1)
		if err := os.Rename(fromPath, toPath); err != nil && !os.IsNotExist(err) {
			return errors.Wrap(err, "error rotating old log entries")
		}
	}

	if err := os.Rename(name, name+".1"); err != nil && !os.IsNotExist(err) {
		return errors.Wrap(err, "error rotating current log")
	}
	return nil
}

// LogPath returns the location the given writer logs to.
func (w *RotateFileWriter) LogPath() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.f.Name()
}

// MaxFiles return maximum number of files
func (w *RotateFileWriter) MaxFiles() int {
	return w.maxFiles
}

//NotifyRotate returns the new subscriber
func (w *RotateFileWriter) NotifyRotate() chan interface{} {
	return w.notifyRotate.Subscribe()
}

//NotifyRotateEvict removes the specified subscriber from receiving any more messages.
func (w *RotateFileWriter) NotifyRotateEvict(sub chan interface{}) {
	w.notifyRotate.Evict(sub)
}

// Close closes underlying file and signals all readers to stop.
func (w *RotateFileWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return nil
	}
	if err := w.f.Close(); err != nil {
		return err
	}
	w.closed = true
	return nil
}
