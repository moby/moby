// +build windows

package server

import (
	"errors"
	"net"
	"net/http"
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
				Handler: s.CreateMux(),
			},
			l,
		})
	}
	return res, nil

}

func allocateDaemonPort(addr string) error {
	return nil
}
