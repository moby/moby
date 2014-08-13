package log

import (
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
)

type priority int

const (
	errorFormat = "[%s] %s:%d %s\n"
	logFormat   = "[%s] %s\n"

	fatal priority = iota
	error
	info
	debug
)

// A common interface to access the Fatal method of
// both testing.B and testing.T.
type Fataler interface {
	Fatal(args ...interface{})
}

func (p priority) String() string {
	switch p {
	case fatal:
		return "fatal"
	case error:
		return "error"
	case info:
		return "info"
	case debug:
		return "debug"
	}

	return ""
}

// Debug function, if the debug flag is set, then display. Do nothing otherwise
// If Docker is in damon mode, also send the debug info on the socket
func Debugf(format string, a ...interface{}) {
	if os.Getenv("DEBUG") != "" {
		logf(os.Stderr, debug, format, a...)
	}
}

func Infof(format string, a ...interface{}) {
	logf(os.Stdout, info, format, a...)
}

func Errorf(format string, a ...interface{}) {
	logf(os.Stderr, error, format, a...)
}

func Fatalf(format string, a ...interface{}) {
	logf(os.Stderr, fatal, format, a...)
	os.Exit(1)
}

func logf(stream io.Writer, level priority, format string, a ...interface{}) {
	var prefix string

	if level <= error || level == debug {
		// Retrieve the stack infos
		_, file, line, ok := runtime.Caller(2)
		if !ok {
			file = "<unknown>"
			line = -1
		} else {
			file = file[strings.LastIndex(file, "/")+1:]
		}
		prefix = fmt.Sprintf(errorFormat, level.String(), file, line, format)
	} else {
		prefix = fmt.Sprintf(logFormat, level.String(), format)
	}

	fmt.Fprintf(stream, prefix, a...)
}
