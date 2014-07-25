package log

import (
	"runtime"
	"strings"
	"os"
	"fmt"
	"log"
	"io"
)

const (
	format = "[%s] %s:%d %s\n"
)

type logger struct {
	*log.Logger
}

func New(out io.Writer, prefix string, flag int) *logger {
	return &logger{Logger: log.New(out, prefix, flag)}
}

// Debug function, if the debug flag is set, then display. Do nothing otherwise
// If Docker is in damon mode, also send the debug info on the socket
func Debugf(format string, a ...interface{}) {
	if os.Getenv("DEBUG") != "" {
		logf(os.Stderr, "debug", format, a...)
	}
}

func Infof(format string, a ...interface{}) {
	logf(os.Stdout, "info", format, a...)
}

func Errorf(format string, a ...interface{}) {
	logf(os.Stderr, "error", format, a...)
}

func Fatalf(format string, a ...interface{}) {
	logf(os.Stderr, "fatal", format, a...)
	os.Exit(1)
}

func logf(stream io.Writer, level string, format string, a ...interface{}) {
	// Retrieve the stack infos
	_, file, line, ok := runtime.Caller(2)
	if !ok {
		file = "<unknown>"
		line = -1
	} else {
		file = file[strings.LastIndex(file, "/")+1:]
	}

	fmt.Fprintf(stream, fmt.Sprintf(format, level, file, line, format), a...)
}
