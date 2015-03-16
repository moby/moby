// +build windows

package server

import (
	"fmt"

	"github.com/docker/docker/engine"
)

// NewServer sets up the required Server and does protocol specific checking.
func NewServer(proto, addr string, job *engine.Job) (Server, error) {
	// Basic error and sanity checking
	switch proto {
	case "tcp":
		return setupTcpHttp(addr, job)
	default:
		return nil, errors.New("Invalid protocol format. Windows only supports tcp.")
	}
}

// Called through eng.Job("acceptconnections")
func AcceptConnections(job *engine.Job) engine.Status {

	// close the lock so the listeners start accepting connections
	if activationLock != nil {
		close(activationLock)
	}

	return engine.StatusOK
}
