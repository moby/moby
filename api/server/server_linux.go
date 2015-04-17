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
func NewServer(proto, addr string, conf *ServerConfig, eng *engine.Engine) (Server, error) {
	var (
		err error
		l   net.Listener
		r   = createRouter(
			eng,
			conf.Logging,
			conf.EnableCors,
			conf.CorsHeaders,
			conf.Version,
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
		if !conf.TlsVerify {
			logrus.Warn("/!\\ DON'T BIND ON ANY IP ADDRESS WITHOUT setting -tlsverify IF YOU DON'T KNOW WHAT YOU'RE DOING /!\\")
		}
		if l, err = NewTcpSocket(addr, tlsConfigFromServerConfig(conf)); err != nil {
			return nil, err
		}
		if err := allocateDaemonPort(addr); err != nil {
			return nil, err
		}
	case "unix":
		if l, err = NewUnixSocket(addr, conf.SocketGroup); err != nil {
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

func AcceptConnections() {
	// Tell the init daemon we are accepting requests
	go systemd.SdNotify("READY=1")
	// close the lock so the listeners start accepting connections
	select {
	case <-activationLock:
	default:
		close(activationLock)
	}
}
