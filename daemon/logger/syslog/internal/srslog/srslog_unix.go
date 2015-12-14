package srslog

import (
	"errors"
	"fmt"
	"net"
	"os"
	"time"
)

// unixSyslog opens a connection to the syslog daemon running on the
// local machine using a Unix domain socket.

func unixSyslog() (conn serverConn, err error) {
	logTypes := []string{"unixgram", "unix"}
	logPaths := []string{"/dev/log", "/var/run/syslog", "/var/run/log"}
	for _, network := range logTypes {
		for _, path := range logPaths {
			conn, err := net.Dial(network, path)
			if err != nil {
				continue
			} else {
				return &localConn{conn: conn}, nil
			}
		}
	}
	return nil, errors.New("Unix syslog delivery error")
}

type localConn struct {
	conn net.Conn
}

func (n *localConn) writeString(p Priority, hostname, tag, msg string) error {
	// Compared to the network form at srslog.netConn, the changes are:
	//	1. Use time.Stamp instead of time.RFC3339.
	//	2. Drop the hostname field from the Fprintf.
	timestamp := time.Now().Format(time.Stamp)
	_, err := fmt.Fprintf(n.conn, "<%d>%s %s[%d]: %s",
		p, timestamp,
		tag, os.Getpid(), msg)
	return err
}

func (n *localConn) close() error {
	return n.conn.Close()
}
