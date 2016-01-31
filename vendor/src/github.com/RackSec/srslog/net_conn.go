package srslog

import (
	"fmt"
	"net"
	"os"
	"time"
)

type netConn struct {
	conn net.Conn
}

func (n *netConn) writeString(p Priority, hostname, tag, msg string) error {
	timestamp := time.Now().Format(time.RFC3339)
	_, err := fmt.Fprintf(n.conn, "<%d>%s %s %s[%d]: %s",
		p, timestamp, hostname,
		tag, os.Getpid(), msg)
	return err
}

func (n *netConn) close() error {
	return n.conn.Close()
}
