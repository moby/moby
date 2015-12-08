package srslog

import (
	"errors"
	"fmt"
	"net"
)

// unixSyslog opens a connection to the syslog daemon running on the
// local machine using a Unix domain socket.

func unixSyslog() (conn serverConn, err error) {
	fmt.Println("unixSyslog")
	logTypes := []string{"unixgram", "unix"}
	logPaths := []string{"/dev/log", "/var/run/syslog", "/var/run/log"}
	for _, network := range logTypes {
		for _, path := range logPaths {
			fmt.Println("dialing", network, path)
			conn, err := net.Dial(network, path)
			if err != nil {
				fmt.Println("failed:", err)
				continue
			} else {
				fmt.Println("success", network, path)
				return &netConn{conn: conn, local: true}, nil
			}
		}
	}
	fmt.Println("no unixSyslog")
	return nil, errors.New("Unix syslog delivery error")
}
