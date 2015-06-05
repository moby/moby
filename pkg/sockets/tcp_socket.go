package sockets

import (
	"crypto/tls"
	"net"

	"github.com/docker/docker/pkg/listenbuffer"
)

func NewTcpSocket(addr string, tlsConfig *tls.Config, activate <-chan struct{}) (net.Listener, error) {
	l, err := listenbuffer.NewListenBuffer("tcp", addr, activate)
	if err != nil {
		return nil, err
	}
	if tlsConfig != nil {
		tlsConfig.NextProtos = []string{"http/1.1"}
		l = tls.NewListener(l, tlsConfig)
	}
	return l, nil
}
