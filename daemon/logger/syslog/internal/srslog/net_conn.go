package srslog

import (
	"fmt"
	"net"
	"os"
	"time"
)

// netConn has an internal net.Conn and adheres to the serverConn interface,
// allowing us to send syslog messages over the network.
type netConn struct {
	conn net.Conn
}

// writeString formats syslog messages using time.RFC3339 and includes the
// hostname, and sends the message to the connection.
func (n *netConn) writeString(p Priority, hostname, tag, msg string) error {
	timestamp := time.Now().Format(time.RFC3339)
	_, err := fmt.Fprintf(n.conn, "<%d>%s %s %s[%d]: %s",
		p, timestamp, hostname,
		tag, os.Getpid(), msg)
	return err
}

// close the network connection
func (n *netConn) close() error {
	return n.conn.Close()
}
