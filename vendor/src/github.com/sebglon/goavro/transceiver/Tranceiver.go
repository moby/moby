package transceiver

import (
	"bytes"
	"io"
	"time"
)

type WriteHandshake func() ([]byte, error)
type ReadHandshake func(io.Reader) (bool, error)
type Transceiver interface {
	Transceive(request []bytes.Buffer) ([]io.Reader, error)
	InitHandshake(WriteHandshake, ReadHandshake )
	Close()



}



type Config struct {
	Port       int           `json:"port"`
	Host       string        `json:"host"`
	Network    string        `json:"network"`
	SocketPath string        `json:"socket_path"`
	Timeout          time.Duration `json:"timeout"`
	AsyncConnect     bool          `json:"async_connect"`
	BufferLimit      int           `json:"buffer_limit"`
	RetryWait        int           `json:"retry_wait"`
	MaxRetry         int           `json:"max_retry"`
	InitialCap	 int		`json:"initial_cap"`
	MaxCap		 int		`json:"max_cap"`
}

