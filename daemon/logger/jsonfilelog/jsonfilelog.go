package jsonfilelog

import (
	"bytes"
	"io"
	"os"
	"sync"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/daemon/logger"
	"github.com/docker/docker/pkg/jsonlog"
	"github.com/docker/docker/pkg/timeutils"
)

const (
	Name = "json-file"
)

// JSONFileLogger is Logger implementation for default docker logging:
// JSON objects to file
type JSONFileLogger struct {
	buf *bytes.Buffer
	f   *os.File   // store for closing
	mu  sync.Mutex // protects buffer

	ctx logger.Context
}

func init() {
	if err := logger.RegisterLogDriver(Name, New); err != nil {
		logrus.Fatal(err)
	}
}

// New creates new JSONFileLogger which writes to filename
func New(ctx logger.Context) (logger.Logger, error) {
	log, err := os.OpenFile(ctx.LogPath, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0600)
	if err != nil {
		return nil, err
	}
	return &JSONFileLogger{
		f:   log,
		buf: bytes.NewBuffer(nil),
		ctx: ctx,
	}, nil
}

// Log converts logger.Message to jsonlog.JSONLog and serializes it to file
func (l *JSONFileLogger) Log(msg *logger.Message) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	timestamp, err := timeutils.FastMarshalJSON(msg.Timestamp)
	if err != nil {
		return err
	}
	err = (&jsonlog.JSONLogBytes{Log: append(msg.Line, '\n'), Stream: msg.Source, Created: timestamp}).MarshalJSONBuf(l.buf)
	if err != nil {
		return err
	}
	l.buf.WriteByte('\n')
	_, err = l.buf.WriteTo(l.f)
	if err != nil {
		// this buffer is screwed, replace it with another to avoid races
		l.buf = bytes.NewBuffer(nil)
		return err
	}
	return nil
}

func (l *JSONFileLogger) GetReader() (io.Reader, error) {
	return os.Open(l.ctx.LogPath)
}

func (l *JSONFileLogger) LogPath() string {
	return l.ctx.LogPath
}

// Close closes underlying file
func (l *JSONFileLogger) Close() error {
	return l.f.Close()
}

// Name returns name of this logger
func (l *JSONFileLogger) Name() string {
	return Name
}
