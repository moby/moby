package jsonfilelog

import (
	"bytes"
	"os"

	"github.com/docker/docker/daemon/logger"
	"github.com/docker/docker/pkg/jsonlog"
)

// JSONFileLogger is Logger implementation for default docker logging:
// JSON objects to file
type JSONFileLogger struct {
	buf *bytes.Buffer
	f   *os.File // store for closing
}

// New creates new JSONFileLogger which writes to filename
func New(filename string) (logger.Logger, error) {
	log, err := os.OpenFile(filename, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0600)
	if err != nil {
		return nil, err
	}
	return &JSONFileLogger{
		f:   log,
		buf: bytes.NewBuffer(nil),
	}, nil
}

// Log converts logger.Message to jsonlog.JSONLog and serializes it to file
func (l *JSONFileLogger) Log(msg *logger.Message) error {
	err := (&jsonlog.JSONLog{Log: string(msg.Line) + "\n", Stream: msg.Source, Created: msg.Timestamp}).MarshalJSONBuf(l.buf)
	if err != nil {
		return err
	}
	l.buf.WriteByte('\n')
	_, err = l.buf.WriteTo(l.f)
	return err
}

// Close closes underlying file
func (l *JSONFileLogger) Close() error {
	return l.f.Close()
}

// Name returns name of this logger
func (l *JSONFileLogger) Name() string {
	return "JSONFile"
}
