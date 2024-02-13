// Package jsonfilelog provides the default Logger implementation for
// Docker logging. This logger logs to files on the host server in the
// JSON format.
package jsonfilelog // import "github.com/docker/docker/daemon/logger/jsonfilelog"

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"sync"

	"github.com/docker/docker/daemon/logger"
	"github.com/docker/docker/daemon/logger/jsonfilelog/jsonlog"
	"github.com/docker/docker/daemon/logger/loggerutils"
	units "github.com/docker/go-units"
	"github.com/pkg/errors"
)

// Name is the name of the file that the jsonlogger logs to.
const Name = "json-file"

// Every buffer will have to store the same constant json structure with the message
// len(`{"log":"","stream:"stdout","time":"2000-01-01T00:00:00.000000000Z"}\n`) = 68.
// So let's start with a buffer bigger than this.
const initialBufSize = 256

var buffersPool = sync.Pool{New: func() interface{} { return bytes.NewBuffer(make([]byte, 0, initialBufSize)) }}

// JSONFileLogger is Logger implementation for default Docker logging.
type JSONFileLogger struct {
	writer *loggerutils.LogFile
	tag    string // tag values requested by the user to log
	extra  json.RawMessage
}

func init() {
	if err := logger.RegisterLogDriver(Name, New); err != nil {
		panic(err)
	}
	if err := logger.RegisterLogOptValidator(Name, ValidateLogOpt); err != nil {
		panic(err)
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
	maxFiles := 1
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

	var extra json.RawMessage
	if len(attrs) > 0 {
		var err error
		extra, err = json.Marshal(attrs)
		if err != nil {
			return nil, err
		}
	}

	writer, err := loggerutils.NewLogFile(info.LogPath, capval, maxFiles, compress, decodeFunc, 0o640, getTailReader)
	if err != nil {
		return nil, err
	}

	return &JSONFileLogger{
		writer: writer,
		tag:    tag,
		extra:  extra,
	}, nil
}

// Log converts logger.Message to jsonlog.JSONLog and serializes it to file.
func (l *JSONFileLogger) Log(msg *logger.Message) error {
	buf := buffersPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer buffersPool.Put(buf)

	timestamp := msg.Timestamp
	err := marshalMessage(msg, l.extra, buf)
	logger.PutMessage(msg)

	if err != nil {
		return err
	}

	return l.writer.WriteLogEntry(timestamp, buf.Bytes())
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
	return l.writer.Close()
}

// Name returns name of this logger.
func (l *JSONFileLogger) Name() string {
	return Name
}
