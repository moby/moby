package rcli

import (
	"io"
	"io/ioutil"
	"net"
	"log"
	"fmt"
	"encoding/json"
	"bufio"
)

func CallTCP(addr string, args ...string) (*net.TCPConn, error) {
	cmd, err := json.Marshal(args)
	if err != nil {
		return nil, err
	}
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}
	if _, err := fmt.Fprintln(conn, string(cmd)); err != nil {
		return nil, err
	}
	return conn.(*net.TCPConn), nil
}

func ListenAndServeTCP(addr string, service Service) error {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	defer listener.Close()
	for {
		if conn, err := listener.Accept(); err != nil {
			return err
		} else {
			go func() {
				if err := Serve(conn, service); err != nil {
					log.Printf("Error: " + err.Error() + "\n")
					fmt.Fprintf(conn, "Error: " + err.Error() + "\n")
				}
				conn.Close()
			}()
		}
	}
	return nil
}

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

