// +build experimental

package system

import (
	"fmt"
	"net"
	"net/http"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/api/server/httputils"
	"github.com/docker/docker/pkg/ioutils"
	"golang.org/x/net/context"
	"golang.org/x/net/websocket"
)

func (s *systemRouter) getWSTunnelTCPPort(r *http.Request) (int, error) {
	port := httputils.Int64ValueOrZero(r, "port")
	// TODO: assert that the port is published by the daemon
	if port == 0 || port > 65535 {
		return 0, fmt.Errorf("invalid tcp port %d", port)
	}
	return int(port), nil
}

func tunnelHandler(tcpConn net.Conn) func(wsConn *websocket.Conn) {
	return func(wsConn *websocket.Conn) {
		ioutils.BidirectionalCopy(tcpConn, wsConn,
			func(err error, direction ioutils.Direction) {
				logrus.Warnf(
					"error while copying (%v): %v",
					direction, err)
			})
	}
}

func (s *systemRouter) wsTunnel(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := httputils.ParseForm(r); err != nil {
		return err
	}
	tcpPort, err := s.getWSTunnelTCPPort(r)
	if err != nil {
		return err
	}
	tcpConn, err := net.Dial("tcp", fmt.Sprintf(":%d", tcpPort))
	if err != nil {
		return err
	}
	srv := websocket.Server{Handler: tunnelHandler(tcpConn)}
	srv.ServeHTTP(w, r)
	return nil
}
