package rcli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
)

// Connect to a remote endpoint using protocol `proto` and address `addr`,
// issue a single call, and return the result.
// `proto` may be "tcp", "unix", etc. See the `net` package for available protocols.
func Call(proto, addr string, args ...string) (*net.TCPConn, error) {
	cmd, err := json.Marshal(args)
	if err != nil {
		return nil, err
	}
	conn, err := net.Dial(proto, addr)
	if err != nil {
		return nil, err
	}
	if _, err := fmt.Fprintln(conn, string(cmd)); err != nil {
		return nil, err
	}
	return conn.(*net.TCPConn), nil
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
			go func() {
				if err := Serve(conn, service); err != nil {
					log.Printf("Error: " + err.Error() + "\n")
					fmt.Fprintf(conn, "Error: "+err.Error()+"\n")
				}
				conn.Close()
			}()
		}
	}
	return nil
}

// Parse an rcli call on a new connection, and pass it to `service` if it
// is valid.
func Serve(conn io.ReadWriter, service Service) error {
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
