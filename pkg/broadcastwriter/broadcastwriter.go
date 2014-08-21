package broadcastwriter

import (
	"bytes"
	"encoding/json"
	"io"
	"sync"
	"time"

	"github.com/docker/docker/pkg/jsonlog"
	"github.com/docker/docker/pkg/log"
)

// BroadcastWriter accumulate multiple io.WriteCloser by stream.
type BroadcastWriter struct {
	sync.Mutex
	buf     *bytes.Buffer
	streams map[string](map[io.WriteCloser]struct{})
}

// AddWriter adds new io.WriteCloser for stream.
// If stream is "", then all writes proceed as is. Otherwise every line from
// input will be packed to serialized jsonlog.JSONLog.
func (w *BroadcastWriter) AddWriter(writer io.WriteCloser, stream string) {
	w.Lock()
	if _, ok := w.streams[stream]; !ok {
		w.streams[stream] = make(map[io.WriteCloser]struct{})
	}
	w.streams[stream][writer] = struct{}{}
	w.Unlock()
}

// Write writes bytes to all writers. Failed writers will be evicted during
// this call.
func (w *BroadcastWriter) Write(p []byte) (n int, err error) {
	created := time.Now().UTC()
	w.Lock()
	if writers, ok := w.streams[""]; ok {
		for sw := range writers {
			if n, err := sw.Write(p); err != nil || n != len(p) {
				// On error, evict the writer
				delete(writers, sw)
			}
		}
	}
	w.buf.Write(p)
	for {
		line, err := w.buf.ReadString('\n')
		if err != nil {
			w.buf.Write([]byte(line))
			break
		}
		for stream, writers := range w.streams {
			if stream == "" {
				continue
			}
			b, err := json.Marshal(jsonlog.JSONLog{Log: line, Stream: stream, Created: created})
			if err != nil {
				log.Errorf("Error making JSON log line: %s", err)
				continue
			}
			b = append(b, '\n')
			for sw := range writers {
				if _, err := sw.Write(b); err != nil {
					delete(writers, sw)
				}
			}
		}
	}
	w.Unlock()
	return len(p), nil
}

// Clean closes and removes all writers. Last non-eol-terminated part of data
// will be saved.
func (w *BroadcastWriter) Clean() error {
	w.Lock()
	for _, writers := range w.streams {
		for w := range writers {
			w.Close()
		}
	}
	w.streams = make(map[string](map[io.WriteCloser]struct{}))
	w.Unlock()
	return nil
}

func New() *BroadcastWriter {
	return &BroadcastWriter{
		streams: make(map[string](map[io.WriteCloser]struct{})),
		buf:     bytes.NewBuffer(nil),
	}
}
