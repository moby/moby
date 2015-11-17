package loggerutils

import (
	"os"
	"strconv"
	"sync"

	"github.com/docker/docker/pkg/pubsub"
)

// RotateFileWriter is Logger implementation for default Docker logging.
type RotateFileWriter struct {
	f            *os.File // store for closing
	mu           sync.Mutex
	capacity     int64 //maximum size of each file
	maxFiles     int   //maximum number of files
	notifyRotate *pubsub.Publisher
}

//NewRotateFileWriter creates new RotateFileWriter
func NewRotateFileWriter(logPath string, capacity int64, maxFiles int) (*RotateFileWriter, error) {
	log, err := os.OpenFile(logPath, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0640)
	if err != nil {
		return &RotateFileWriter{}, err
	}

	return &RotateFileWriter{
		f:            log,
		capacity:     capacity,
		maxFiles:     maxFiles,
		notifyRotate: pubsub.NewPublisher(0, 1),
	}, nil
}

//WriteLog write log messge to File
func (w *RotateFileWriter) Write(message []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if err := w.checkCapacityAndRotate(); err != nil {
		return -1, err
	}

	return w.f.Write(message)
}

func (w *RotateFileWriter) checkCapacityAndRotate() error {
	if w.capacity == -1 {
		return nil
	}

	meta, err := w.f.Stat()
	if err != nil {
		return err
	}

	if meta.Size() >= w.capacity {
		name := w.f.Name()
		if err := w.f.Close(); err != nil {
			return err
		}
		if err := rotate(name, w.maxFiles); err != nil {
			return err
		}
		file, err := os.OpenFile(name, os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 06400)
		if err != nil {
			return err
		}
		w.f = file
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
		if err := backup(fromPath, toPath); err != nil && !os.IsNotExist(err) {
			return err
		}
	}

	if err := backup(name, name+".1"); err != nil {
		return err
	}
	return nil
}

// backup renames a file from fromPath to toPath
func backup(fromPath, toPath string) error {
	if _, err := os.Stat(fromPath); os.IsNotExist(err) {
		return err
	}

	if _, err := os.Stat(toPath); !os.IsNotExist(err) {
		err := os.Remove(toPath)
		if err != nil {
			return err
		}
	}

	return os.Rename(fromPath, toPath)
}

// LogPath returns the location the given wirter logs to.
func (w *RotateFileWriter) LogPath() string {
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
	return w.f.Close()
}
