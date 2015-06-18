package logger

import (
	"errors"
	"io"
	"time"
)

var ReadLogsNotSupported = errors.New("configured logging reader does not support reading")

// Message is datastructure that represents record from some container
type Message struct {
	ContainerID string
	Line        []byte
	Source      string
	Timestamp   time.Time
}

// Logger is interface for docker logging drivers
type Logger interface {
	Log(*Message) error
	Name() string
	Close() error
	GetReader() (io.Reader, error)
}
