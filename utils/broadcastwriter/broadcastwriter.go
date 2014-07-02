package broadcastwriter

import (
	"bytes"
	"encoding/json"
	"io"
	"sync"
	"time"

	"github.com/dotcloud/docker/utils"
)

type BroadcastWriter struct {
	sync.Mutex
	buf     *bytes.Buffer
	streams map[string](map[io.WriteCloser]struct{})
}

func (w *BroadcastWriter) AddWriter(writer io.WriteCloser, stream string) {
	w.Lock()
	if _, ok := w.streams[stream]; !ok {
		w.streams[stream] = make(map[io.WriteCloser]struct{})
	}
	w.streams[stream][writer] = struct{}{}
	w.Unlock()
}

func (w *BroadcastWriter) Write(p []byte) (n int, err error) {
	created := time.Now().UTC()
	w.Lock()
	defer w.Unlock()
	if writers, ok := w.streams[""]; ok {
		for sw := range writers {
			if n, err := sw.Write(p); err != nil || n != len(p) {
				// On error, evict the writer
				delete(writers, sw)
			}
		}
	}
	w.buf.Write(p)
	lines := []string{}
	for {
		line, err := w.buf.ReadString('\n')
		if err != nil {
			w.buf.Write([]byte(line))
			break
		}
		lines = append(lines, line)
	}

	if len(lines) != 0 {
		for stream, writers := range w.streams {
			if stream == "" {
				continue
			}
			var lp []byte
			for _, line := range lines {
				b, err := json.Marshal(&utils.JSONLog{Log: line, Stream: stream, Created: created})
				if err != nil {
					utils.Errorf("Error making JSON log line: %s", err)
				}
				lp = append(lp, b...)
				lp = append(lp, '\n')
			}
			for sw := range writers {
				if _, err := sw.Write(lp); err != nil {
					delete(writers, sw)
				}
			}
		}
	}
	return len(p), nil
}

func (w *BroadcastWriter) Close() error {
	w.Lock()
	defer w.Unlock()
	for _, writers := range w.streams {
		for w := range writers {
			w.Close()
		}
	}
	w.streams = make(map[string](map[io.WriteCloser]struct{}))
	return nil
}

func New() *BroadcastWriter {
	return &BroadcastWriter{
		streams: make(map[string](map[io.WriteCloser]struct{})),
		buf:     bytes.NewBuffer(nil),
	}
}
