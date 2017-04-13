package devicemapper

import (
	"fmt"

	"github.com/Sirupsen/logrus"
)

// definitions from lvm2 lib/log/log.h
const (
	LogLevelFatal  LogLevel = 2 + iota // _LOG_FATAL
	LogLevelErr                        // _LOG_ERR
	LogLevelWarn                       // _LOG_WARN
	LogLevelNotice                     // _LOG_NOTICE
	LogLevelInfo                       // _LOG_INFO
	LogLevelDebug                      // _LOG_DEBUG
)

// Working "notice" level into mappings as that is not a direct mapping for logrus log levels.
var logFunc = []func(format string, args ...interface{}){
	logrus.Infof,
	logrus.Infof,
	logrus.Fatalf,
	logrus.Errorf,
	logrus.Warnf,
	logrus.Infof,
	logrus.Infof,
	logrus.Debugf,
}

// LogLevel represents the level of logging to be included from the library
type LogLevel int

// LogLevelValue takes a logrus.Level and returns the libdevicemapper log level constant.
func LogLevelValue(lvl logrus.Level) (LogLevel, error) {
	switch lvl {
	case logrus.FatalLevel:
		return LogLevelFatal, nil
	case logrus.ErrorLevel:
		return LogLevelErr, nil
	case logrus.WarnLevel:
		return LogLevelWarn, nil
	case logrus.InfoLevel:
		return LogLevelInfo, nil
	case logrus.DebugLevel:
		return LogLevelDebug, nil
	}

	return LogLevelInfo, fmt.Errorf("not a valid libdevicemapper log level: %q", lvl)
}

func (lvl LogLevel) String() string {
	switch lvl {
	case LogLevelDebug:
		return "debug"
	case LogLevelInfo:
		return "info"
	case LogLevelNotice:
		return "notice"
	case LogLevelWarn:
		return "warning"
	case LogLevelErr:
		return "error"
	case LogLevelFatal:
		return "fatal"
	}

	return "unknown"
}

// Logf formats and logs a message from libdevmapper
func Logf(level LogLevel, file string, line int, dmError int, message string) {
	f := logrus.Debugf
	if level < LogLevelDebug {
		f = logFunc[level]
	}

	// Level included to document the libdevmapper logging level vs. the logrus level
	f("libdevmapper(%s): %s:%d (%d) %s", level, file, line, dmError, message)
}
