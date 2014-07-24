package log

import (
	"runtime"
	"strings"
	"os"
	"fmt"
	"log"
	"io"
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
		logf("debug", format, a...)
	}
}

func Infof(format string, a ...interface{}) {
	logf("info", format, a...)
}

func Errorf(format string, a ...interface{}) {
	logf("error", format, a...)
}

func Fatalf(format string, a ...interface{}) {
	logf("fatal", format, a...)
	os.Exit(1)
}

func logf(level string, format string, a ...interface{}) {
	// Retrieve the stack infos
	_, file, line, ok := runtime.Caller(2)
	if !ok {
		file = "<unknown>"
		line = -1
	} else {
		file = file[strings.LastIndex(file, "/")+1:]
	}

	fmt.Fprintf(os.Stderr, fmt.Sprintf("[%s] %s:%d %s\n", level, file, line, format), a...)
}
