// +build linux

package server

import (
	"fmt"
	"net/http"
	"os"
	"syscall"

	"github.com/docker/docker/engine"
)

// NewServer sets up the required Server and does protocol specific checking.
func NewServer(proto, addr string, job *engine.Job) (Server, error) {
	// Basic error and sanity checking
	switch proto {
	case "fd":
		return nil, serveFd(addr, job)
	case "tcp":
		return setupTcpHttp(addr, job)
	case "unix":
		return setupUnixHttp(addr, job)
	default:
		return nil, fmt.Errorf("Invalid protocol format.")
	}
}

func setupUnixHttp(addr string, job *engine.Job) (*HttpServer, error) {
    r := createRouter(job.Eng, job.GetenvBool("Logging"), job.GetenvBool("EnableCors"), job.Getenv("CorsHeaders"), job.Getenv("Version"))

	if err := syscall.Unlink(addr); err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	mask := syscall.Umask(0777)
	defer syscall.Umask(mask)

	l, err := newListener("unix", addr, job.GetenvBool("BufferRequests"))
	if err != nil {
		return nil, err
	}

	if err := setSocketGroup(addr, job.Getenv("SocketGroup")); err != nil {
		return nil, err
	}

	if err := os.Chmod(addr, 0660); err != nil {
		return nil, err
	}

	return &HttpServer{&http.Server{Addr: addr, Handler: r}, l}, nil
}
