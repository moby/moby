package funker

import (
	"encoding/json"
	"io/ioutil"
	"net"
	"time"
)

// Call a Funker function
func Call(name string, args interface{}) (interface{}, error) {
	argsJSON, err := json.Marshal(args)
	if err != nil {
		return nil, err
	}

	addr, err := net.ResolveTCPAddr("tcp", name+":9999")
	if err != nil {
		return nil, err
	}

	conn, err := net.DialTCP("tcp", nil, addr)
	if err != nil {
		return nil, err
	}
	// Keepalive is a workaround for docker/docker#29655 .
	// The implementation of FIN_WAIT2 seems weird on Swarm-mode.
	// It seems always refuseing any packet after 60 seconds.
	//
	// TODO: remove this workaround if the issue gets resolved on the Docker side
	if err := conn.SetKeepAlive(true); err != nil {
		return nil, err
	}
	if err := conn.SetKeepAlivePeriod(30 * time.Second); err != nil {
		return nil, err
	}
	if _, err = conn.Write(argsJSON); err != nil {
		return nil, err
	}
	if err = conn.CloseWrite(); err != nil {
		return nil, err
	}
	retJSON, err := ioutil.ReadAll(conn)
	if err != nil {
		return nil, err
	}
	var ret interface{}
	err = json.Unmarshal(retJSON, &ret)
	return ret, err
}
