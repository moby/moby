// Package jsonfilelog provides the default Logger implementation for
// Docker logging. This logger logs to files on the host server in the
// JSON format.
package jsonfilelog // import "github.com/moby/moby/daemon/logger/jsonfilelog"

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"sync"

	"github.com/moby/moby/daemon/logger"
	"github.com/moby/moby/daemon/logger/jsonfilelog/jsonlog"
	"github.com/moby/moby/daemon/logger/loggerutils"
	units "github.com/docker/go-units"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// Name is the name of the file that the jsonlogger logs to.
const Name = "json-file"

// JSONFileLogger is Logger implementation for default Docker logging.
type JSONFileLogger struct {
	mu      sync.Mutex
	closed  bool
	writer  *loggerutils.LogFile
	readers map[*logger.LogWatcher]struct{} // stores the active log followers
	tag     string                          // tag values requested by the user to log
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
func New(info logger.Info) (logger.Logger, error) {
	var capval int64 = -1
	if capacity, ok := info.Config["max-size"]; ok {
		var err error
		capval, err = units.FromHumanSize(capacity)
		if err != nil {
			return nil, err
		}
		if capval <= 0 {
			return nil, fmt.Errorf("max-size must be a positive number")
		}
	}
	var maxFiles = 1
	if maxFileString, ok := info.Config["max-file"]; ok {
		var err error
		maxFiles, err = strconv.Atoi(maxFileString)
		if err != nil {
			return nil, err
		}
		if maxFiles < 1 {
			return nil, fmt.Errorf("max-file cannot be less than 1")
		}
	}

	var compress bool
	if compressString, ok := info.Config["compress"]; ok {
		var err error
		compress, err = strconv.ParseBool(compressString)
		if err != nil {
			return nil, err
		}
		if compress && (maxFiles == 1 || capval == -1) {
			return nil, fmt.Errorf("compress cannot be true when max-file is less than 2 or max-size is not set")
		}
	}

	attrs, err := info.ExtraAttributes(nil)
	if err != nil {
		return nil, err
	}

	// no default template. only use a tag if the user asked for it
	tag, err := loggerutils.ParseLogTag(info, "")
	if err != nil {
		return nil, err
	}
	if tag != "" {
		attrs["tag"] = tag
	}

	var extra []byte
	if len(attrs) > 0 {
		var err error
		extra, err = json.Marshal(attrs)
		if err != nil {
			return nil, err
		}
	}

	buf := bytes.NewBuffer(nil)
	marshalFunc := func(msg *logger.Message) ([]byte, error) {
		if err := marshalMessage(msg, extra, buf); err != nil {
			return nil, err
		}
		b := buf.Bytes()
		buf.Reset()
		return b, nil
	}

	writer, err := loggerutils.NewLogFile(info.LogPath, capval, maxFiles, compress, marshalFunc, decodeFunc, 0640, getTailReader)
	if err != nil {
		return nil, err
	}

	return &JSONFileLogger{
		writer:  writer,
		readers: make(map[*logger.LogWatcher]struct{}),
		tag:     tag,
	}, nil
}

// Log converts logger.Message to jsonlog.JSONLog and serializes it to file.
func (l *JSONFileLogger) Log(msg *logger.Message) error {
	l.mu.Lock()
	err := l.writer.WriteLogEntry(msg)
	l.mu.Unlock()
	return err
}

func marshalMessage(msg *logger.Message, extra json.RawMessage, buf *bytes.Buffer) error {
	logLine := msg.Line
	if msg.PLogMetaData == nil || (msg.PLogMetaData != nil && msg.PLogMetaData.Last) {
		logLine = append(msg.Line, '\n')
	}
	err := (&jsonlog.JSONLogs{
		Log:      logLine,
		Stream:   msg.Source,
		Created:  msg.Timestamp,
		RawAttrs: extra,
	}).MarshalJSONBuf(buf)
	if err != nil {
		return errors.Wrap(err, "error writing log message to buffer")
	}
	err = buf.WriteByte('\n')
	return errors.Wrap(err, "error finalizing log buffer")
}

// ValidateLogOpt looks for json specific log options max-file & max-size.
func ValidateLogOpt(cfg map[string]string) error {
	for key := range cfg {
		switch key {
		case "max-file":
		case "max-size":
		case "compress":
		case "labels":
		case "labels-regex":
		case "env":
		case "env-regex":
		case "tag":
		default:
			return fmt.Errorf("unknown log opt '%s' for json-file log driver", key)
		}
	}
	return nil
}

// Close closes underlying file and signals all the readers
// that the logs producer is gone.
func (l *JSONFileLogger) Close() error {
	l.mu.Lock()
	l.closed = true
	err := l.writer.Close()
	for r := range l.readers {
		r.ProducerGone()
		delete(l.readers, r)
	}
	l.mu.Unlock()
	return err
}

// Name returns name of this logger.
func (l *JSONFileLogger) Name() string {
	return Name
}
