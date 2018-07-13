package gelf

import (
	"fmt"
	"net"
	"os"
	"sync"
	"time"
)

const (
	DefaultMaxReconnect   = 3
	DefaultReconnectDelay = 1
)

type TCPWriter struct {
	GelfWriter
	mu             sync.Mutex
	MaxReconnect   int
	ReconnectDelay time.Duration
}

func NewTCPWriter(addr string) (*TCPWriter, error) {
	var err error
	w := new(TCPWriter)
	w.MaxReconnect = DefaultMaxReconnect
	w.ReconnectDelay = DefaultReconnectDelay
	w.proto = "tcp"
	w.addr = addr

	if w.conn, err = net.Dial("tcp", addr); err != nil {
		return nil, err
	}
	if w.hostname, err = os.Hostname(); err != nil {
		return nil, err
	}

	return w, nil
}

// WriteMessage sends the specified message to the GELF server
// specified in the call to New().  It assumes all the fields are
// filled out appropriately.  In general, clients will want to use
// Write, rather than WriteMessage.
func (w *TCPWriter) WriteMessage(m *Message) (err error) {
	messageBytes, err := m.toBytes()
	if err != nil {
		return err
	}

	messageBytes = append(messageBytes, 0)

	n, err := w.writeToSocketWithReconnectAttempts(messageBytes)
	if err != nil {
		return err
	}
	if n != len(messageBytes) {
		return fmt.Errorf("bad write (%d/%d)", n, len(messageBytes))
	}

	return nil
}

func (w *TCPWriter) Write(p []byte) (n int, err error) {
	file, line := getCallerIgnoringLogMulti(1)

	m := constructMessage(p, w.hostname, w.Facility, file, line)

	if err = w.WriteMessage(m); err != nil {
		return 0, err
	}

	return len(p), nil
}

func (w *TCPWriter) writeToSocketWithReconnectAttempts(zBytes []byte) (n int, err error) {
	var errConn error
	var i int

	w.mu.Lock()
	for i = 0; i <= w.MaxReconnect; i++ {
		errConn = nil

		if w.conn != nil {
			n, err = w.conn.Write(zBytes)
		} else {
			err = fmt.Errorf("Connection was nil, will attempt reconnect")
		}
		if err != nil {
			time.Sleep(w.ReconnectDelay * time.Second)
			w.conn, errConn = net.Dial("tcp", w.addr)
		} else {
			break
		}
	}
	w.mu.Unlock()

	if i > w.MaxReconnect {
		return 0, fmt.Errorf("Maximum reconnection attempts was reached; giving up")
	}
	if errConn != nil {
		return 0, fmt.Errorf("Write Failed: %s\nReconnection failed: %s", err, errConn)
	}
	return n, nil
}
