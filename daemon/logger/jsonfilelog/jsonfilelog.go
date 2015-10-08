// Package jsonfilelog provides the default Logger implementation for
// Docker logging. This logger logs to files on the host server in the
// JSON format.
package jsonfilelog

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"sync"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/daemon/logger"
	"github.com/docker/docker/pkg/jsonlog"
	"github.com/docker/docker/pkg/pubsub"
	"github.com/docker/docker/pkg/timeutils"
	"github.com/docker/docker/pkg/units"
)

// Name is the name of the file that the jsonlogger logs to.
const Name = "json-file"

// JSONFileLogger is Logger implementation for default Docker logging.
type JSONFileLogger struct {
	buf          *bytes.Buffer
	f            *os.File   // store for closing
	mu           sync.Mutex // protects buffer
	capacity     int64      //maximum size of each file
	n            int        //maximum number of files
	ctx          logger.Context
	readers      map[*logger.LogWatcher]struct{} // stores the active log followers
	notifyRotate *pubsub.Publisher
	extra        []byte // json-encoded extra attributes
}

func init() {
	if err := logger.RegisterLogDriver(Name, New); err != nil {
		logrus.Fatal(err)
	}
	if err := logger.RegisterLogOptValidator(Name, ValidateLogOpt); err != nil {
		logrus.Fatal(err)
	}
}

// New creates new JSONFileLogger which writes to filename passed in
// on given context.
func New(ctx logger.Context) (logger.Logger, error) {
	log, err := os.OpenFile(ctx.LogPath, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0600)
	if err != nil {
		return nil, err
	}
	var capval int64 = -1
	if capacity, ok := ctx.Config["max-size"]; ok {
		var err error
		capval, err = units.FromHumanSize(capacity)
		if err != nil {
			return nil, err
		}
	}
	var maxFiles = 1
	if maxFileString, ok := ctx.Config["max-file"]; ok {
		maxFiles, err = strconv.Atoi(maxFileString)
		if err != nil {
			return nil, err
		}
		if maxFiles < 1 {
			return nil, fmt.Errorf("max-file cannot be less than 1")
		}
	}

	var extra []byte
	if attrs := ctx.ExtraAttributes(nil); len(attrs) > 0 {
		var err error
		extra, err = json.Marshal(attrs)
		if err != nil {
			return nil, err
		}
	}

	return &JSONFileLogger{
		f:            log,
		buf:          bytes.NewBuffer(nil),
		ctx:          ctx,
		capacity:     capval,
		n:            maxFiles,
		readers:      make(map[*logger.LogWatcher]struct{}),
		notifyRotate: pubsub.NewPublisher(0, 1),
		extra:        extra,
	}, nil
}

// Log converts logger.Message to jsonlog.JSONLog and serializes it to file.
func (l *JSONFileLogger) Log(msg *logger.Message) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	timestamp, err := timeutils.FastMarshalJSON(msg.Timestamp)
	if err != nil {
		return err
	}
	err = (&jsonlog.JSONLogs{
		Log:      append(msg.Line, '\n'),
		Stream:   msg.Source,
		Created:  timestamp,
		RawAttrs: l.extra,
	}).MarshalJSONBuf(l.buf)
	if err != nil {
		return err
	}
	l.buf.WriteByte('\n')
	_, err = writeLog(l)
	return err
}

func writeLog(l *JSONFileLogger) (int64, error) {
	if l.capacity == -1 {
		return writeToBuf(l)
	}
	meta, err := l.f.Stat()
	if err != nil {
		return -1, err
	}
	if meta.Size() >= l.capacity {
		name := l.f.Name()
		if err := l.f.Close(); err != nil {
			return -1, err
		}
		if err := rotate(name, l.n); err != nil {
			return -1, err
		}
		file, err := os.OpenFile(name, os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0666)
		if err != nil {
			return -1, err
		}
		l.f = file
		l.notifyRotate.Publish(struct{}{})
	}
	return writeToBuf(l)
}

func writeToBuf(l *JSONFileLogger) (int64, error) {
	i, err := l.buf.WriteTo(l.f)
	if err != nil {
		l.buf = bytes.NewBuffer(nil)
	}
	return i, err
}

func rotate(name string, n int) error {
	if n < 2 {
		return nil
	}
	for i := n - 1; i > 1; i-- {
		oldFile := name + "." + strconv.Itoa(i)
		replacingFile := name + "." + strconv.Itoa(i-1)
		if err := backup(oldFile, replacingFile); err != nil {
			return err
		}
	}
	if err := backup(name+".1", name); err != nil {
		return err
	}
	return nil
}

// backup renames a file from curr to old, creating an empty file curr if it does not exist.
func backup(old, curr string) error {
	if _, err := os.Stat(old); !os.IsNotExist(err) {
		err := os.Remove(old)
		if err != nil {
			return err
		}
	}
	if _, err := os.Stat(curr); os.IsNotExist(err) {
		f, err := os.Create(curr)
		if err != nil {
			return err
		}
		f.Close()
	}
	return os.Rename(curr, old)
}

// ValidateLogOpt looks for json specific log options max-file & max-size.
func ValidateLogOpt(cfg map[string]string) error {
	for key := range cfg {
		switch key {
		case "max-file":
		case "max-size":
		case "labels":
		case "env":
		default:
			return fmt.Errorf("unknown log opt '%s' for json-file log driver", key)
		}
	}
	return nil
}

// LogPath returns the location the given json logger logs to.
func (l *JSONFileLogger) LogPath() string {
	return l.ctx.LogPath
}

// Close closes underlying file and signals all readers to stop.
func (l *JSONFileLogger) Close() error {
	l.mu.Lock()
	err := l.f.Close()
	for r := range l.readers {
		r.Close()
		delete(l.readers, r)
	}
	l.mu.Unlock()
	return err
}

// Name returns name of this logger.
func (l *JSONFileLogger) Name() string {
	return Name
}
