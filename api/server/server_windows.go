// +build windows

package server

import (
	"errors"
	"net"
	"net/http"

	"github.com/docker/docker/daemon"
)

// NewServer sets up the required Server and does protocol specific checking.
func (s *Server) newServer(proto, addr string) ([]serverCloser, error) {
	var (
		ls []net.Listener
	)
	switch proto {
	case "tcp":
		l, err := s.initTCPSocket(addr)
		if err != nil {
			return nil, err
		}
		ls = append(ls, l)

	default:
		return nil, errors.New("Invalid protocol format. Windows only supports tcp.")
	}

	var res []serverCloser
	for _, l := range ls {
		res = append(res, &HTTPServer{
			&http.Server{
				Addr:    addr,
				Handler: s.router,
			},
			l,
		})
	}
	return res, nil

}

// AcceptConnections allows router to start listening for the incoming requests.
func (s *Server) AcceptConnections(d *daemon.Daemon) {
	s.daemon = d
	s.registerSubRouter()
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

// getContainersByNameDownlevel performs processing for pre 1.20 APIs. This
// is only relevant on non-Windows daemons.
func getContainersByNameDownlevel(w http.ResponseWriter, s *Server, namevar string) error {
	return nil
}
