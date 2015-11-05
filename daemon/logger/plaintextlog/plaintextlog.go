package plaintextlog

import (
	"bytes"
	"fmt"
	"os"
	"strconv"
	"sync"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/daemon/logger"
	"github.com/docker/docker/pkg/units"
)

const (
	driverName = "plain-text"
)

type plainTextLogger struct {
	buf      *bytes.Buffer
	f        *os.File   // store for closing
	mu       sync.Mutex // protects buffer
	capacity int64      //maximum size of each file
	n        int        //maximum number of files
	ctx      logger.Context
}

func init() {
	if err := logger.RegisterLogDriver(driverName, New); err != nil {
		logrus.Fatal(err)
	}
	if err := logger.RegisterLogOptValidator(driverName, ValidateLogOpt); err != nil {
		logrus.Fatal(err)
	}
}

// New creates new plainTextLogger which writes to filename passed in
// on given context.
func New(ctx logger.Context) (logger.Logger, error) {
	log, err := os.OpenFile(ctx.Config["log-path"], os.O_RDWR|os.O_APPEND|os.O_CREATE, 0600)
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

	return &plainTextLogger{
		f:        log,
		buf:      bytes.NewBuffer(nil),
		ctx:      ctx,
		capacity: capval,
		n:        maxFiles,
	}, nil
}

func (l *plainTextLogger) Log(msg *logger.Message) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	logString := fmt.Sprintf("%s : %s\n", msg.Timestamp.String(), string(msg.Line))
	l.buf.WriteString(logString)

	_, err := writeLog(l)
	return err
}

func writeLog(l *plainTextLogger) (int64, error) {
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
	}
	return writeToBuf(l)
}

func writeToBuf(l *plainTextLogger) (int64, error) {
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

// ValidateLogOpt looks for log-path, max-size, max-file
func ValidateLogOpt(cfg map[string]string) error {
	for key := range cfg {
		switch key {
		case "log-path":
		case "max-file":
		case "max-size":
		default:
			return fmt.Errorf("unknown log opt '%s' for plain-text log driver", key)
		}
	}

	if cfg["log-path"] == "" {
		return fmt.Errorf("must specify a value for log-path")
	}

	return nil
}

// Close closes underlying file and signals all readers to stop.
func (l *plainTextLogger) Close() error {
	l.mu.Lock()
	err := l.f.Close()
	l.mu.Unlock()
	return err
}

// Name returns name of this logger.
func (l *plainTextLogger) Name() string {
	return driverName
}
