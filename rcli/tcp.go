package rcli

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
)

// Note: the globals are here to avoid import cycle
// FIXME: Handle debug levels mode?
var DEBUG_FLAG bool = false
var CLIENT_SOCKET io.Writer = nil

type DockerTCPConn struct {
	conn       *net.TCPConn
	options    *DockerConnOptions
	optionsBuf *[]byte
	handshaked bool
	client     bool
}

func NewDockerTCPConn(conn *net.TCPConn, client bool) *DockerTCPConn {
	return &DockerTCPConn{
		conn:    conn,
		options: &DockerConnOptions{},
		client:  client,
	}
}

func (c *DockerTCPConn) SetOptionRawTerminal() {
	c.options.RawTerminal = true
}

func (c *DockerTCPConn) GetOptions() *DockerConnOptions {
	if c.client && !c.handshaked {
		// Attempt to parse options encoded as a JSON dict and store
		// the reminder of what we read from the socket in a buffer.
		//
		// bufio (and its ReadBytes method) would have been nice here,
		// but if json.Unmarshal() fails (which will happen if we speak
		// to a version of docker that doesn't send any option), then
		// we can't put the data back in it for the next Read().
		c.handshaked = true
		buf := make([]byte, 4096)
		if n, _ := c.conn.Read(buf); n > 0 {
			buf = buf[:n]
			if nl := bytes.IndexByte(buf, '\n'); nl != -1 {
				if err := json.Unmarshal(buf[:nl], c.options); err == nil {
					buf = buf[nl+1:]
				}
			}
			c.optionsBuf = &buf
		}
	}

	return c.options
}

func (c *DockerTCPConn) Read(b []byte) (int, error) {
	if c.optionsBuf != nil {
		// Consume what we buffered in GetOptions() first:
		optionsBuf := *c.optionsBuf
		optionsBuflen := len(optionsBuf)
		copied := copy(b, optionsBuf)
		if copied < optionsBuflen {
			optionsBuf = optionsBuf[copied:]
			c.optionsBuf = &optionsBuf
			return copied, nil
		}
		c.optionsBuf = nil
		return copied, nil
	}
	return c.conn.Read(b)
}

func (c *DockerTCPConn) Write(b []byte) (int, error) {
	optionsLen := 0
	if !c.client && !c.handshaked {
		c.handshaked = true
		options, _ := json.Marshal(c.options)
		options = append(options, '\n')
		if optionsLen, err := c.conn.Write(options); err != nil {
			return optionsLen, err
		}
	}
	n, err := c.conn.Write(b)
	return n + optionsLen, err
}

func (c *DockerTCPConn) Flush() error {
	_, err := c.conn.Write([]byte{})
	return err
}

func (c *DockerTCPConn) Close() error { return c.conn.Close() }

func (c *DockerTCPConn) CloseWrite() error { return c.conn.CloseWrite() }

func (c *DockerTCPConn) CloseRead() error { return c.conn.CloseRead() }

// Connect to a remote endpoint using protocol `proto` and address `addr`,
// issue a single call, and return the result.
// `proto` may be "tcp", "unix", etc. See the `net` package for available protocols.
func Call(proto, addr string, args ...string) (DockerConn, error) {
	cmd, err := json.Marshal(args)
	if err != nil {
		return nil, err
	}
	conn, err := dialDocker(proto, addr)
	if err != nil {
		return nil, err
	}
	if _, err := fmt.Fprintln(conn, string(cmd)); err != nil {
		return nil, err
	}
	return conn, nil
}

// Listen on `addr`, using protocol `proto`, for incoming rcli calls,
// and pass them to `service`.
func ListenAndServe(proto, addr string, service Service) error {
	listener, err := net.Listen(proto, addr)
	if err != nil {
		return err
	}
	log.Printf("Listening for RCLI/%s on %s\n", proto, addr)
	defer listener.Close()
	for {
		if conn, err := listener.Accept(); err != nil {
			return err
		} else {
			conn, err := newDockerServerConn(conn)
			if err != nil {
				return err
			}
			go func() {
				if DEBUG_FLAG {
					CLIENT_SOCKET = conn
				}
				if err := Serve(conn, service); err != nil {
					log.Println("Error:", err.Error())
					fmt.Fprintln(conn, "Error:", err.Error())
				}
				conn.Close()
			}()
		}
	}
	return nil
}

// Parse an rcli call on a new connection, and pass it to `service` if it
// is valid.
func Serve(conn DockerConn, service Service) error {
	r := bufio.NewReader(conn)
	var args []string
	if line, err := r.ReadString('\n'); err != nil {
		return err
	} else if err := json.Unmarshal([]byte(line), &args); err != nil {
		return err
	} else {
		return call(service, ioutil.NopCloser(r), conn, args...)
	}
	return nil
}
