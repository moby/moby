// +build linux

package server

import (
	"fmt"
	"net"
	"net/http"

	"github.com/docker/docker/daemon"
	"github.com/docker/docker/pkg/sockets"
	"github.com/docker/docker/pkg/systemd"
)

// newServer sets up the required serverCloser and does protocol specific checking.
func (s *Server) newServer(proto, addr string) (serverCloser, error) {
	var (
		err error
		l   net.Listener
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
		<-s.start
		// Since ListenFD will return one or more sockets we have
		// to create a go func to spawn off multiple serves
		for i := range ls {
			listener := ls[i]
			go func() {
				httpSrv := http.Server{Handler: s.router}
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
		l, err = s.initTcpSocket(addr)
		if err != nil {
			return nil, err
		}
	case "unix":
		if l, err = sockets.NewUnixSocket(addr, s.cfg.SocketGroup, s.start); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("Invalid protocol format: %q", proto)
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
	// Tell the init daemon we are accepting requests
	s.daemon = d
	go systemd.SdNotify("READY=1")
	// close the lock so the listeners start accepting connections
	select {
	case <-s.start:
	default:
		close(s.start)
	}
}
