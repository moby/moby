// +build windows
package server

import (
	"errors"
	"net"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/engine"
)

// NewServer sets up the required Server and does protocol specific checking.
func NewServer(proto, addr string, job *engine.Job) (Server, error) {
	var (
		err error
		l   net.Listener
		r   = createRouter(
			job.Eng,
			job.GetenvBool("Logging"),
			job.GetenvBool("EnableCors"),
			job.Getenv("CorsHeaders"),
			job.Getenv("Version"),
		)
	)
	switch proto {
	case "tcp":
		if !job.GetenvBool("TlsVerify") {
			logrus.Infof("/!\\ DON'T BIND ON ANY IP ADDRESS WITHOUT setting -tlsverify IF YOU DON'T KNOW WHAT YOU'RE DOING /!\\")
		}
		if l, err = NewTcpSocket(addr, tlsConfigFromJob(job)); err != nil {
			return nil, err
		}
		if err := allocateDaemonPort(addr); err != nil {
			return nil, err
		}
	default:
		return nil, errors.New("Invalid protocol format. Windows only supports tcp.")
	}
}

// Called through eng.Job("acceptconnections")
func AcceptConnections(job *engine.Job) error {
	// close the lock so the listeners start accepting connections
	select {
	case <-activationLock:
	default:
		close(activationLock)
	}
	return nil
}
