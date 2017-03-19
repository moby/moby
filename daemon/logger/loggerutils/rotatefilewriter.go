package loggerutils

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/docker/docker/daemon/logger"
	"github.com/docker/docker/pkg/jsonlog"
	"github.com/docker/docker/pkg/pubsub"
	"github.com/docker/docker/pkg/tailfile"
)

// LogFileMetaData is used to store metadata of all json log files
type LogFileMetaData struct {
	FirstLogTime time.Time
	LastLogTime  time.Time
}

// RotateFileWriter is Logger implementation for default Docker logging.
type RotateFileWriter struct {
	f            *os.File // store for closing
	mu           sync.Mutex
	capacity     int64 //maximum size of each file
	currentSize  int64 // current size of the latest file
	maxFiles     int   //maximum number of files
	notifyRotate *pubsub.Publisher
	meta         []*LogFileMetaData
}

//NewRotateFileWriter creates new RotateFileWriter
func NewRotateFileWriter(logPath string, capacity int64, maxFiles int) (*RotateFileWriter, error) {
	log, err := os.OpenFile(logPath, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0640)
	if err != nil {
		return nil, err
	}

	size, err := log.Seek(0, os.SEEK_END)
	if err != nil {
		return nil, err
	}

	meta, err := genLogMetaInfo(log.Name(), maxFiles)
	if err != nil {
		return nil, err
	}

	return &RotateFileWriter{
		f:            log,
		capacity:     capacity,
		currentSize:  size,
		maxFiles:     maxFiles,
		notifyRotate: pubsub.NewPublisher(0, 1),
		meta:         meta,
	}, nil
}

//WriteLog write log message to File
func (w *RotateFileWriter) Write(message []byte) (int, error) {
	w.mu.Lock()
	if err := w.checkCapacityAndRotate(); err != nil {
		w.mu.Unlock()
		return -1, err
	}

	n, err := w.f.Write(message)
	if err == nil {
		w.currentSize += int64(n)
	}
	w.mu.Unlock()
	return n, err
}

func (w *RotateFileWriter) checkCapacityAndRotate() error {
	if w.capacity == -1 {
		return nil
	}

	meta := w.meta[0]
	if meta == nil {
		meta = &LogFileMetaData{}
		w.meta[0] = meta
	}

	if w.currentSize == 0 {
		meta.FirstLogTime = time.Now().UTC()
	}

	if w.currentSize >= w.capacity {
		meta.LastLogTime = time.Now().UTC()
		if err := w.f.Close(); err != nil {
			return err
		}
		name := w.f.Name()
		if err := rotate(w.meta, name, w.maxFiles); err != nil {
			return err
		}
		file, err := os.OpenFile(name, os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 06400)
		if err != nil {
			return err
		}
		w.f = file
		w.currentSize = 0
		w.notifyRotate.Publish(struct{}{})
	}

	return nil
}

func rotate(meta []*LogFileMetaData, name string, maxFiles int) error {
	if maxFiles < 2 {
		return nil
	}

	for i := maxFiles - 1; i > 1; i-- {
		if meta[i-1] == nil {
			continue
		}

		toPath := name + "." + strconv.Itoa(i)
		fromPath := name + "." + strconv.Itoa(i-1)
		meta[i] = meta[i-1]

		if err := os.Rename(fromPath, toPath); err != nil {
			meta[i] = nil
			if !os.IsNotExist(err) {
				return err
			}
		}
	}

	meta[1] = meta[0]
	if err := os.Rename(name, name+".1"); err != nil {
		meta[1] = nil
		if !os.IsNotExist(err) {
			return err
		}
	}
	meta[0] = nil
	return nil
}

func genLogMetaInfo(name string, maxFiles int) ([]*LogFileMetaData, error) {
	metaList := make([]*LogFileMetaData, maxFiles)
	for i := maxFiles; i > 1; i-- {
		meta := &LogFileMetaData{}
		path := name + "." + strconv.Itoa(i-1)
		f, err := os.Open(path)
		if err != nil {
			if !os.IsNotExist(err) {
				return nil, err
			}
			continue
		}
		defer f.Close()

		// get the first log timestamp
		reader := bufio.NewReader(f)
		first, err := reader.ReadBytes('\n')
		if err != nil {
			return nil, err
		}
		rdr := bytes.NewBuffer(first)
		dec := json.NewDecoder(rdr)
		l := &jsonlog.JSONLog{}
		msg, err := DecodeLogLine(dec, l)
		if err != nil {
			return nil, err
		}
		meta.FirstLogTime = msg.Timestamp

		// get the last log timestamp
		f.Seek(0, os.SEEK_SET)
		ll, err := tailfile.TailFile(f, 1)
		if err != nil {
			return nil, err
		}
		rdr = bytes.NewBuffer(bytes.Join(ll, []byte("\n")))
		dec = json.NewDecoder(rdr)
		msg, err = DecodeLogLine(dec, l)
		if err != nil {
			return nil, err
		}
		meta.LastLogTime = msg.Timestamp
		metaList[i-1] = meta
	}
	return metaList, nil
}

// DecodeLogLine decodes one line of json-file log
func DecodeLogLine(dec *json.Decoder, l *jsonlog.JSONLog) (*logger.Message, error) {
	l.Reset()
	if err := dec.Decode(l); err != nil {
		return nil, err
	}
	msg := &logger.Message{
		Source:    l.Stream,
		Timestamp: l.Created,
		Line:      []byte(l.Log),
		Attrs:     l.Attrs,
	}
	return msg, nil
}

// LogPath returns the location the given writer logs to.
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

// MetaData return log files meta data
func (w *RotateFileWriter) MetaData() []*LogFileMetaData {
	return w.meta
}
