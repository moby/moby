package listeners

import (
	"crypto/tls"
	"net"

	"github.com/Sirupsen/logrus"
	"github.com/docker/go-connections/sockets"
)

func initTCPSocket(addr string, tlsConfig *tls.Config) (l net.Listener, err error) {
	if tlsConfig == nil || tlsConfig.ClientAuth != tls.RequireAndVerifyClientCert {
		// TODO: Move this outside pkg/listeners since it's Docker-specific.
		//       ... and slightly scary.
		logrus.Warn("/!\\ DON'T BIND ON ANY IP ADDRESS WITHOUT setting -tlsverify IF YOU DON'T KNOW WHAT YOU'RE DOING /!\\")
	}
	if l, err = sockets.NewTCPSocket(addr, tlsConfig); err != nil {
		return nil, err
	}
	if err := allocateDaemonPort(addr); err != nil {
		return nil, err
	}
	return
}
