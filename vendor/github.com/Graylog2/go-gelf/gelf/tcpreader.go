package gelf

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
)

type TCPReader struct {
	listener *net.TCPListener
	conn     net.Conn
	messages chan []byte
}

func newTCPReader(addr string) (*TCPReader, chan string, error) {
	var err error
	tcpAddr, err := net.ResolveTCPAddr("tcp", addr)
	if err != nil {
		return nil, nil, fmt.Errorf("ResolveTCPAddr('%s'): %s", addr, err)
	}

	listener, err := net.ListenTCP("tcp", tcpAddr)
	if err != nil {
		return nil, nil, fmt.Errorf("ListenTCP: %s", err)
	}

	r := &TCPReader{
		listener: listener,
		messages: make(chan []byte, 100), // Make a buffered channel with at most 100 messages
	}

	signal := make(chan string, 1)

	go r.listenUntilCloseSignal(signal)

	return r, signal, nil
}

func (r *TCPReader) listenUntilCloseSignal(signal chan string) {
	defer func() { signal <- "done" }()
	defer r.listener.Close()
	for {
		conn, err := r.listener.Accept()
		if err != nil {
			break
		}
		go handleConnection(conn, r.messages)
		select {
		case sig := <-signal:
			if sig == "stop" {
				break
			}
		default:
		}
	}
}

func (r *TCPReader) addr() string {
	return r.listener.Addr().String()
}

func handleConnection(conn net.Conn, messages chan<- []byte) {
	defer conn.Close()
	reader := bufio.NewReader(conn)

	var b []byte
	var err error

	for {
		if b, err = reader.ReadBytes(0); err != nil {
			continue
		}
		if len(b) > 0 {
			messages <- b
		}
	}
}

func (r *TCPReader) readMessage() (*Message, error) {
	b := <-r.messages

	var msg Message
	if err := json.Unmarshal(b[:len(b)-1], &msg); err != nil {
		return nil, fmt.Errorf("json.Unmarshal: %s", err)
	}

	return &msg, nil
}

func (r *TCPReader) Close() {
	r.listener.Close()
}
