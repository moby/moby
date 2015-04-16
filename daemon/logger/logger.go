package logger

import "time"

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
}
