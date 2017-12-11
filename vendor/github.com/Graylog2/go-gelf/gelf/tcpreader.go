package gelf

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"time"
)

type TCPReader struct {
	listener *net.TCPListener
	conn     net.Conn
	messages chan []byte
}

type connChannels struct {
	drop    chan string
	confirm chan string
}

func newTCPReader(addr string) (*TCPReader, chan string, chan string, error) {
	var err error
	tcpAddr, err := net.ResolveTCPAddr("tcp", addr)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("ResolveTCPAddr('%s'): %s", addr, err)
	}

	listener, err := net.ListenTCP("tcp", tcpAddr)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("ListenTCP: %s", err)
	}

	r := &TCPReader{
		listener: listener,
		messages: make(chan []byte, 100), // Make a buffered channel with at most 100 messages
	}

	closeSignal := make(chan string, 1)
	doneSignal := make(chan string, 1)

	go r.listenUntilCloseSignal(closeSignal, doneSignal)

	return r, closeSignal, doneSignal, nil
}

func (r *TCPReader) accepter(connections chan net.Conn) {
	for {
		conn, err := r.listener.Accept()
		if err != nil {
			break
		}
		connections <- conn
	}
}

func (r *TCPReader) listenUntilCloseSignal(closeSignal chan string, doneSignal chan string) {
	defer func() { doneSignal <- "done" }()
	defer r.listener.Close()
	var conns []connChannels
	connectionsChannel := make(chan net.Conn, 1)
	go r.accepter(connectionsChannel)
	for {
		select {
		case conn := <-connectionsChannel:
			dropSignal := make(chan string, 1)
			dropConfirm := make(chan string, 1)
			channels := connChannels{drop: dropSignal, confirm: dropConfirm}
			go handleConnection(conn, r.messages, dropSignal, dropConfirm)
			conns = append(conns, channels)
		default:
		}

		select {
		case sig := <-closeSignal:
			if sig == "stop" || sig == "drop" {
				if len(conns) >= 1 {
					for _, s := range conns {
						if s.drop != nil {
							s.drop <- "drop"
							<-s.confirm
							conns = append(conns[:0], conns[1:]...)
						}
					}
					if sig == "stop" {
						return
					}
				} else if sig == "stop" {
					closeSignal <- "stop"
				}
				if sig == "drop" {
					doneSignal <- "done"
				}
			}
		default:
		}
	}
}

func (r *TCPReader) addr() string {
	return r.listener.Addr().String()
}

func handleConnection(conn net.Conn, messages chan<- []byte, dropSignal chan string, dropConfirm chan string) {
	defer func() { dropConfirm <- "done" }()
	defer conn.Close()
	reader := bufio.NewReader(conn)

	var b []byte
	var err error
	drop := false
	canDrop := false

	for {
		conn.SetDeadline(time.Now().Add(2 * time.Second))
		if b, err = reader.ReadBytes(0); err != nil {
			if drop {
				return
			}
		} else if len(b) > 0 {
			messages <- b
			canDrop = true
			if drop {
				return
			}
		} else if drop {
			return
		}
		select {
		case sig := <-dropSignal:
			if sig == "drop" {
				drop = true
				time.Sleep(1 * time.Second)
				if canDrop {
					return
				}
			}
		default:
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
