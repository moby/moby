// +build linux

package server

import (
	"fmt"
	"net/http"
	"os"
	"syscall"

	"github.com/docker/docker/engine"
	"github.com/docker/docker/pkg/systemd"
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

// serveFd creates an http.Server and sets it up to serve given a socket activated
// argument.
func serveFd(addr string, job *engine.Job) error {
	r := createRouter(job.Eng, job.GetenvBool("Logging"), job.GetenvBool("EnableCors"), job.Getenv("CorsHeaders"), job.Getenv("Version"))

	ls, e := systemd.ListenFD(addr)
	if e != nil {
		return e
	}

	chErrors := make(chan error, len(ls))

	// We don't want to start serving on these sockets until the
	// daemon is initialized and installed. Otherwise required handlers
	// won't be ready.
	<-activationLock

	// Since ListenFD will return one or more sockets we have
	// to create a go func to spawn off multiple serves
	for i := range ls {
		listener := ls[i]
		go func() {
			httpSrv := http.Server{Handler: r}
			chErrors <- httpSrv.Serve(listener)
		}()
	}

	for i := 0; i < len(ls); i++ {
		err := <-chErrors
		if err != nil {
			return err
		}
	}

	return nil
}

// Called through eng.Job("acceptconnections")
func AcceptConnections(job *engine.Job) engine.Status {
	// Tell the init daemon we are accepting requests
	go systemd.SdNotify("READY=1")

	// close the lock so the listeners start accepting connections
	if activationLock != nil {
		close(activationLock)
	}

	return engine.StatusOK
}
