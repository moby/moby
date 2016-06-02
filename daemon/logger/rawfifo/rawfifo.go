// +build !windows

// Package rawfifo provides the logdriver for forwarding logs to named pipes.
package rawfifo

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"syscall"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/daemon/logger"
)

const (
	name          = "rawfifo"
	keyRawfifoDir = "rawfifo-dir"
)

type rawfifoLogger struct {
	dir     string
	mu      sync.Mutex
	writers map[string]io.WriteCloser
}

func init() {
	if err := logger.RegisterLogDriver(name, New); err != nil {
		logrus.Fatal(err)
	}
	if err := logger.RegisterLogOptValidator(name, ValidateLogOpt); err != nil {
		logrus.Fatal(err)
	}
}

// New creates new rawfifoLogger.
func New(ctx logger.Context) (logger.Logger, error) {
	dir, ok := ctx.Config[keyRawfifoDir]
	if !ok {
		return nil, fmt.Errorf("logger option %s is not set", keyRawfifoDir)
	}

	if _, err := os.Stat(dir); os.IsNotExist(err) {
		logrus.Debugf("Creating %s", dir)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, err
		}
	} else {
		logrus.Debugf("Reusing %s", dir)
	}
	return &rawfifoLogger{
		dir:     dir,
		writers: make(map[string]io.WriteCloser, 0),
	}, nil
}

func (l *rawfifoLogger) RawWriter(name string) (io.WriteCloser, error) {
	// if an user specify --log-opt="rawfifo-dir=/tmp/foo",
	// fname becomes /tmp/t1/{stdout, stderr}.
	// how to determine fname is not fixed yet. we need to discuss it.

	fname := filepath.Join(l.dir, name)
	var w io.WriteCloser
	l.mu.Lock()
	w, ok := l.writers[fname]
	defer l.mu.Unlock()
	if ok {
		logrus.Debugf("Returning existing Writer for %s", fname)
		return w, nil
	}

	logrus.Debugf("Creating fifo %s", fname)
	if err := syscall.Mkfifo(fname, 0700); err != nil {
		return nil, fmt.Errorf("mkfifo: %s %v", fname, err)
	}
	w, err := os.OpenFile(fname, syscall.O_WRONLY, 0)
	if err != nil {
		return nil, err
	}
	l.writers[fname] = w
	logrus.Debugf("Returning Writer for %s", fname)
	return w, nil
}

func (l *rawfifoLogger) Close() error {
	var errors []error
	l.mu.Lock()
	defer l.mu.Unlock()
	for fname, w := range l.writers {
		logrus.Debugf("Closing Writer for %s", fname)
		err := w.Close()
		logrus.Debugf("Closed Writer for %s(err=%v)", fname, err)
		if err != nil {
			errors = append(errors, err)
		}
		delete(l.writers, fname)
	}

	return fmt.Errorf("error while closing %s: %v", l.Name(), errors)
}

func (l *rawfifoLogger) Name() string {
	return name
}

// ValidateLogOpt looks for rawfifo specific log options,
// rawfifo-dir
func ValidateLogOpt(cfg map[string]string) error {
	for key := range cfg {
		switch key {
		case keyRawfifoDir:
		default:
			return fmt.Errorf("unknown log opt '%s' for rawfifo log driver", key)
		}
	}
	return nil
}
