// +build !windows

package listeners

import (
	"crypto/tls"
	"fmt"
	"net"
	"strconv"

	"github.com/Sirupsen/logrus"
	"github.com/coreos/go-systemd/activation"
	"github.com/docker/go-connections/sockets"
	"github.com/docker/libnetwork/portallocator"
)

// Init creates new listeners for the server.
func Init(proto, addr, socketGroup string, tlsConfig *tls.Config) (ls []net.Listener, err error) {
	switch proto {
	case "fd":
		ls, err = listenFD(addr, tlsConfig)
		if err != nil {
			return nil, err
		}
	case "tcp":
		l, err := initTCPSocket(addr, tlsConfig)
		if err != nil {
			return nil, err
		}
		ls = append(ls, l)
	case "unix":
		l, err := sockets.NewUnixSocket(addr, socketGroup)
		if err != nil {
			return nil, fmt.Errorf("can't create unix socket %s: %v", addr, err)
		}
		ls = append(ls, l)
	default:
		return nil, fmt.Errorf("invalid protocol format: %q", proto)
	}

	return
}

// listenFD returns the specified socket activated files as a slice of
// net.Listeners or all of the activated files if "*" is given.
func listenFD(addr string, tlsConfig *tls.Config) ([]net.Listener, error) {
	var (
		err       error
		listeners []net.Listener
	)
	// socket activation
	if tlsConfig != nil {
		listeners, err = activation.TLSListeners(false, tlsConfig)
	} else {
		listeners, err = activation.Listeners(false)
	}
	if err != nil {
		return nil, err
	}

	if len(listeners) == 0 {
		return nil, fmt.Errorf("no sockets found via socket activation: make sure the service was started by systemd")
	}

	// default to all fds just like unix:// and tcp://
	if addr == "" || addr == "*" {
		return listeners, nil
	}

	fdNum, err := strconv.Atoi(addr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse systemd fd address: should be a number: %v", addr)
	}
	fdOffset := fdNum - 3
	if len(listeners) < int(fdOffset)+1 {
		return nil, fmt.Errorf("too few socket activated files passed in by systemd")
	}
	if listeners[fdOffset] == nil {
		return nil, fmt.Errorf("failed to listen on systemd activated file: fd %d", fdOffset+3)
	}
	for i, ls := range listeners {
		if i == fdOffset || ls == nil {
			continue
		}
		if err := ls.Close(); err != nil {
			// TODO: We shouldn't log inside a library. Remove this or error out.
			logrus.Errorf("failed to close systemd activated file: fd %d: %v", fdOffset+3, err)
		}
	}
	return []net.Listener{listeners[fdOffset]}, nil
}

// allocateDaemonPort ensures that there are no containers
// that try to use any port allocated for the docker server.
// TODO: Move this outside pkg/listeners since it's Docker-specific, and requires
//       libnetwork which increases the dependency tree quite drastically.
func allocateDaemonPort(addr string) error {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return err
	}

	intPort, err := strconv.Atoi(port)
	if err != nil {
		return err
	}

	var hostIPs []net.IP
	if parsedIP := net.ParseIP(host); parsedIP != nil {
		hostIPs = append(hostIPs, parsedIP)
	} else if hostIPs, err = net.LookupIP(host); err != nil {
		return fmt.Errorf("failed to lookup %s address in host specification", host)
	}

	pa := portallocator.Get()
	for _, hostIP := range hostIPs {
		if _, err := pa.RequestPort(hostIP, "tcp", intPort); err != nil {
			return fmt.Errorf("failed to allocate daemon listening port %d (err: %v)", intPort, err)
		}
	}
	return nil
}
