// +build windows

package server

import (
	"errors"
	"net"
	"net/http"

	"github.com/docker/docker/daemon"
)

// NewServer sets up the required Server and does protocol specific checking.
func (s *Server) newServer(proto, addr string) (serverCloser, error) {
	var (
		err error
		l   net.Listener
	)
	switch proto {
	case "tcp":
		l, err = s.initTcpSocket(addr)
		if err != nil {
			return nil, err
		}

	default:
		return nil, errors.New("Invalid protocol format. Windows only supports tcp.")
	}
	return &HttpServer{
		&http.Server{
			Addr:    addr,
			Handler: s.router,
		},
		l,
	}, nil
}

func (s *Server) AcceptConnections(d *daemon.Daemon) {
	s.daemon = d
	// close the lock so the listeners start accepting connections
	select {
	case <-s.start:
	default:
		close(s.start)
	}
}

func allocateDaemonPort(addr string) error {
	return nil
}

func adjustCpuShares(version version.Version, hostConfig *runconfig.HostConfig) {
}
