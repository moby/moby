// +build windows

package server

import (
	"errors"
	"fmt"
	"github.com/Microsoft/go-winio"
	"net"
	"net/http"
	"strings"
)

// NewServer sets up the required Server and does protocol specific checking.
func (s *Server) newServer(proto, addr string) ([]*HTTPServer, error) {
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

	case "npipe":
		// allow Administrators and SYSTEM, plus whatever additional users or groups were specified
		sddl := "D:P(A;;GA;;;BA)(A;;GA;;;SY)"
		if s.cfg.SocketGroup != "" {
			for _, g := range strings.Split(s.cfg.SocketGroup, ",") {
				sid, err := winio.LookupSidByName(g)
				if err != nil {
					return nil, err
				}
				sddl += fmt.Sprintf("(A;;GRGW;;;%s)", sid)
			}
		}
		l, err := winio.ListenPipe(addr, sddl)
		if err != nil {
			return nil, err
		}
		ls = append(ls, l)

	default:
		return nil, errors.New("Invalid protocol format. Windows only supports tcp and npipe.")
	}

	var res []*HTTPServer
	for _, l := range ls {
		res = append(res, &HTTPServer{
			&http.Server{
				Addr: addr,
			},
			l,
		})
	}
	return res, nil

}

func allocateDaemonPort(addr string) error {
	return nil
}
