// Package logger defines interfaces that logger drivers implement to
// log messages.
//
// The other half of a logger driver is the implementation of the
// factory, which holds the contextual instance information that
// allows multiple loggers of the same type to perform different
// actions, such as logging to different locations.
package logger

import (
	"errors"
	"time"

	"github.com/docker/docker/pkg/timeutils"
)

// ErrReadLogsNotSupported is returned when the logger does not support reading logs.
var ErrReadLogsNotSupported = errors.New("configured logging reader does not support reading")

const (
	// TimeFormat is the time format used for timestamps sent to log readers.
	TimeFormat           = timeutils.RFC3339NanoFixed
	logWatcherBufferSize = 4096
)

// Message is datastructure that represents record from some container.
type Message struct {
	ContainerID string
	Line        []byte
	Source      string
	Timestamp   time.Time
}

// Logger is the interface for docker logging drivers.
type Logger interface {
	Log(*Message) error
	Name() string
	Close() error
}

// ReadConfig is the configuration passed into ReadLogs.
type ReadConfig struct {
	Since  time.Time
	Tail   int
	Follow bool
}

// LogReader is the interface for reading log messages for loggers that support reading.
type LogReader interface {
	// Read logs from underlying logging backend
	ReadLogs(ReadConfig) *LogWatcher
}

// LogWatcher is used when consuming logs read from the LogReader interface.
type LogWatcher struct {
	// For sending log messages to a reader.
	Msg chan *Message
	// For sending error messages that occur while while reading logs.
	Err           chan error
	closeNotifier chan struct{}
}

// NewLogWatcher returns a new LogWatcher.
func NewLogWatcher() *LogWatcher {
	return &LogWatcher{
		Msg:           make(chan *Message, logWatcherBufferSize),
		Err:           make(chan error, 1),
		closeNotifier: make(chan struct{}),
	}
}

// Close notifies the underlying log reader to stop.
func (w *LogWatcher) Close() {
	// only close if not already closed
	select {
	case <-w.closeNotifier:
	default:
		close(w.closeNotifier)
	}
}

// WatchClose returns a channel receiver that receives notification
// when the watcher has been closed. This should only be called from
// one goroutine.
func (w *LogWatcher) WatchClose() <-chan struct{} {
	return w.closeNotifier
}
