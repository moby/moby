package log

import (
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/docker/docker/pkg/timeutils"
)

type priority int

const (
	errorFormat = "[%s] [%s] %s:%d %s\n"
	logFormat   = "[%s] [%s] %s\n"

	fatalPriority priority = iota
	errorPriority
	infoPriority
	debugPriority
)

// A common interface to access the Fatal method of
// both testing.B and testing.T.
type Fataler interface {
	Fatal(args ...interface{})
}

func (p priority) String() string {
	switch p {
	case fatalPriority:
		return "fatal"
	case errorPriority:
		return "error"
	case infoPriority:
		return "info"
	case debugPriority:
		return "debug"
	}

	return ""
}

var std = Logger{Out: os.Stdout, Err: os.Stderr}

// Debug function, if the debug flag is set, then display. Do nothing otherwise
// If Docker is in damon mode, also send the debug info on the socket
func Debugf(format string, a ...interface{}) (int, error) {
	if os.Getenv("DEBUG") != "" {
		return std.log(debugPriority, fmt.Sprintf(format, a...))
	}
	return 0, nil
}

func Infof(format string, a ...interface{}) (int, error) {
	return std.log(infoPriority, fmt.Sprintf(format, a...))
}

func Errorf(format string, a ...interface{}) (int, error) {
	return std.log(errorPriority, fmt.Sprintf(format, a...))
}

func Fatal(a ...interface{}) {
	std.log(fatalPriority, fmt.Sprint(a...))
}

func Fatalf(format string, a ...interface{}) {
	std.log(fatalPriority, fmt.Sprintf(format, a...))
}

type Logger struct {
	Err io.Writer
	Out io.Writer
}

func (l Logger) Debugf(format string, a ...interface{}) (int, error) {
	if os.Getenv("DEBUG") != "" {
		return l.log(debugPriority, fmt.Sprintf(format, a))
	}
	return 0, nil
}

func (l Logger) Infof(format string, a ...interface{}) (int, error) {
	return l.log(infoPriority, fmt.Sprintf(format, a...))
}

func (l Logger) Errorf(format string, a ...interface{}) (int, error) {
	return l.log(errorPriority, fmt.Sprintf(format, a...))
}

func (l Logger) Fatalf(format string, a ...interface{}) {
	l.log(fatalPriority, fmt.Sprintf(format, a...))
}

func (l Logger) getStream(level priority) io.Writer {
	switch level {
	case infoPriority:
		return l.Out
	default:
		return l.Err
	}
}

func (l Logger) log(level priority, s string) (int, error) {
	ts := time.Now().Format(timeutils.RFC3339NanoFixed)
	stream := l.getStream(level)
	defer func() {
		if level == fatalPriority {
			os.Exit(1)
		}
	}()
	if level <= errorPriority || level == debugPriority {
		// Retrieve the stack infos
		_, file, line, ok := runtime.Caller(2)
		if !ok {
			file = "<unknown>"
			line = -1
		} else {
			file = file[strings.LastIndex(file, "/")+1:]
		}
		return fmt.Fprintf(stream, errorFormat, ts, level.String(), file, line, s)
	}
	return fmt.Fprintf(stream, logFormat, ts, level.String(), s)
}
