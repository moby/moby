// Package le_go provides a Golang client library for logging to
// logentries.com over a TCP connection.
//
// it uses an access token for sending log events.
package le_go

import (
	"crypto/tls"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"time"
)

// Logger represents a Logentries logger,
// it holds the open TCP connection, access token, prefix and flags.
//
// all Logger operations are thread safe and blocking,
// log operations can be invoked in a non-blocking way by calling them from
// a goroutine.
type Logger struct {
	conn   net.Conn
	flag   int
	mu     sync.Mutex
	prefix string
	token  string
	buf    []byte
}

const lineSep = "\n"

// Connect creates a new Logger instance and opens a TCP connection to
// logentries.com,
// The token can be generated at logentries.com by adding a new log,
// choosing manual configuration and token based TCP connection.
func Connect(token string) (*Logger, error) {
	logger := Logger{
		token: token,
	}

	if err := logger.openConnection(); err != nil {
		return nil, err
	}

	return &logger, nil
}

// Close closes the TCP connection to logentries.com
func (logger *Logger) Close() error {
	if logger.conn != nil {
		return logger.conn.Close()
	}

	return nil
}

// Opens a TCP connection to logentries.com
func (logger *Logger) openConnection() error {
	conn, err := tls.Dial("tcp", "data.logentries.com:443", &tls.Config{})
	if err != nil {
		return err
	}
	logger.conn = conn
	return nil
}

// It returns if the TCP connection to logentries.com is open
func (logger *Logger) isOpenConnection() bool {
	if logger.conn == nil {
		return false
	}

	buf := make([]byte, 1)

	logger.conn.SetReadDeadline(time.Now())

	_, err := logger.conn.Read(buf)

	switch err.(type) {
	case net.Error:
		if err.(net.Error).Timeout() == true {
			logger.conn.SetReadDeadline(time.Time{})

			return true
		}
	}

	return false
}

// It ensures that the TCP connection to logentries.com is open.
// If the connection is closed, a new one is opened.
func (logger *Logger) ensureOpenConnection() error {
	if !logger.isOpenConnection() {
		if err := logger.openConnection(); err != nil {
			return err
		}
	}

	return nil
}

// Fatal is same as Print() but calls to os.Exit(1)
func (logger *Logger) Fatal(v ...interface{}) {
	logger.Output(2, fmt.Sprint(v...))
	os.Exit(1)
}

// Fatalf is same as Printf() but calls to os.Exit(1)
func (logger *Logger) Fatalf(format string, v ...interface{}) {
	logger.Output(2, fmt.Sprintf(format, v...))
	os.Exit(1)
}

// Fatalln is same as Println() but calls to os.Exit(1)
func (logger *Logger) Fatalln(v ...interface{}) {
	logger.Output(2, fmt.Sprintln(v...))
	os.Exit(1)
}

// Flags returns the logger flags
func (logger *Logger) Flags() int {
	return logger.flag
}

// Output does the actual writing to the TCP connection
func (logger *Logger) Output(calldepth int, s string) error {
	_, err := logger.Write([]byte(s))

	return err
}

// Panic is same as Print() but calls to panic
func (logger *Logger) Panic(v ...interface{}) {
	s := fmt.Sprint(v...)
	logger.Output(2, s)
	panic(s)
}

// Panicf is same as Printf() but calls to panic
func (logger *Logger) Panicf(format string, v ...interface{}) {
	s := fmt.Sprintf(format, v...)
	logger.Output(2, s)
	panic(s)
}

// Panicln is same as Println() but calls to panic
func (logger *Logger) Panicln(v ...interface{}) {
	s := fmt.Sprintln(v...)
	logger.Output(2, s)
	panic(s)
}

// Prefix returns the logger prefix
func (logger *Logger) Prefix() string {
	return logger.prefix
}

// Print logs a message
func (logger *Logger) Print(v ...interface{}) {
	logger.Output(2, fmt.Sprint(v...))
}

// Printf logs a formatted message
func (logger *Logger) Printf(format string, v ...interface{}) {
	logger.Output(2, fmt.Sprintf(format, v...))
}

// Println logs a message with a linebreak
func (logger *Logger) Println(v ...interface{}) {
	logger.Output(2, fmt.Sprintln(v...))
}

// SetFlags sets the logger flags
func (logger *Logger) SetFlags(flag int) {
	logger.flag = flag
}

// SetPrefix sets the logger prefix
func (logger *Logger) SetPrefix(prefix string) {
	logger.prefix = prefix
}

// Write writes a bytes array to the Logentries TCP connection,
// it adds the access token and prefix and also replaces
// line breaks with the unicode \u2028 character
func (logger *Logger) Write(p []byte) (n int, err error) {
	if err := logger.ensureOpenConnection(); err != nil {
		return 0, err
	}

	logger.mu.Lock()
	defer logger.mu.Unlock()

	logger.makeBuf(p)

	return logger.conn.Write(logger.buf)
}

// makeBuf constructs the logger buffer
// it is not safe to be used from within multiple concurrent goroutines
func (logger *Logger) makeBuf(p []byte) {
	count := strings.Count(string(p), lineSep)
	p = []byte(strings.Replace(string(p), lineSep, "\u2028", count-1))

	logger.buf = logger.buf[:0]
	logger.buf = append(logger.buf, (logger.token + " ")...)
	logger.buf = append(logger.buf, (logger.prefix + " ")...)
	logger.buf = append(logger.buf, p...)

	if !strings.HasSuffix(string(logger.buf), lineSep) {
		logger.buf = append(logger.buf, (lineSep)...)
	}
}
