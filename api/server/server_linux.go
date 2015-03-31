// +build linux

package server

import (
	"fmt"
	"net"
	"net/http"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/engine"
	"github.com/docker/docker/pkg/systemd"
)

// NewServer sets up the required Server and does protocol specific checking.
func NewServer(proto, addr string, job *engine.Job) (Server, error) {
	var (
		err error
		l   net.Listener
		r   = createRouter(
			job.Eng,
			job.GetenvBool("Logging"),
			job.GetenvBool("EnableCors"),
			job.Getenv("CorsHeaders"),
			job.Getenv("Version"),
		)
	)
	switch proto {
	case "fd":
		ls, err := systemd.ListenFD(addr)
		if err != nil {
			return nil, err
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
			if err := <-chErrors; err != nil {
				return nil, err
			}
		}
		return nil, nil
	case "tcp":
		if !job.GetenvBool("TlsVerify") {
			logrus.Infof("/!\\ DON'T BIND ON ANY IP ADDRESS WITHOUT setting -tlsverify IF YOU DON'T KNOW WHAT YOU'RE DOING /!\\")
		}
		if l, err = NewTcpSocket(addr, tlsConfigFromJob(job)); err != nil {
			return nil, err
		}
		if err := allocateDaemonPort(addr); err != nil {
			return nil, err
		}
	case "unix":
		if l, err = NewUnixSocket(addr, job.Getenv("SocketGroup")); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("Invalid protocol format: %q", proto)
	}
	return &HttpServer{
		&http.Server{
			Addr:    addr,
			Handler: r,
		},
		l,
	}, nil
}

// Called through eng.Job("acceptconnections")
func AcceptConnections(job *engine.Job) error {
	// Tell the init daemon we are accepting requests
	go systemd.SdNotify("READY=1")
	// close the lock so the listeners start accepting connections
	if activationLock != nil {
		close(activationLock)
	}
	return nil
}
