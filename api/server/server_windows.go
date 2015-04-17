// +build windows

package server

import (
	"errors"
	"net"
	"net/http"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/daemon"
)

// NewServer sets up the required Server and does protocol specific checking.
func (s *Server) newServer(proto, addr string) (Server, error) {
	var (
		err error
		l   net.Listener
	)
	switch proto {
	case "tcp":
		if !s.cfg.TlsVerify {
			logrus.Warn("/!\\ DON'T BIND ON ANY IP ADDRESS WITHOUT setting -tlsverify IF YOU DON'T KNOW WHAT YOU'RE DOING /!\\")
		}
		if l, err = NewTcpSocket(addr, tlsConfigFromServerConfig(s.cfg)); err != nil {
			return nil, err
		}
		if err := allocateDaemonPort(addr); err != nil {
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
