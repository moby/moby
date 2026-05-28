package sock

import (
	"fmt"
	"net"

	"github.com/tetratelabs/wazero/experimental/sys"
)

// TCPSock is a pseudo-file representing a TCP socket.
type TCPSock interface {
	sys.File

	Accept() (TCPConn, sys.Errno)
}

// TCPConn is a pseudo-file representing a TCP connection.
type TCPConn interface {
	sys.File

	// Recvfrom only supports the flag sysfs.MSG_PEEK
	// TODO: document this like sys.File with known sys.Errno
	Recvfrom(p []byte, flags int) (n int, errno sys.Errno)

	// TODO: document this like sys.File with known sys.Errno
	Shutdown(how int) sys.Errno
}

// ConfigKey is a context.Context Value key. Its associated value should be a Config.
type ConfigKey struct{}

// Config is an internal struct meant to implement
// the interface in experimental/sock/Config.
type Config struct {
	// TCPAddresses is a slice of the configured host:port pairs.
	TCPAddresses []TCPAddress
}

// TCPAddress is a host:port pair to pre-open.
type TCPAddress struct {
	// Host is the host name for this listener.
	Host string
	// Port is the port number for this listener.
	Port int
}

// WithTCPListener implements the method of the same name in experimental/sock/Config.
//
// However, to avoid cyclic dependencies, this is returning the *Config in this scope.
// The interface is implemented in experimental/sock/Config via delegation.
func (c *Config) WithTCPListener(host string, port int) *Config {
	ret := c.clone()
	ret.TCPAddresses = append(ret.TCPAddresses, TCPAddress{host, port})
	return &ret
}

// Makes a deep copy of this sockConfig.
func (c *Config) clone() Config {
	ret := *c
	ret.TCPAddresses = make([]TCPAddress, 0, len(c.TCPAddresses))
	ret.TCPAddresses = append(ret.TCPAddresses, c.TCPAddresses...)
	return ret
}

// BuildTCPListeners build listeners from the current configuration.
func (c *Config) BuildTCPListeners() (tcpListeners []*net.TCPListener, err error) {
	for _, tcpAddr := range c.TCPAddresses {
		var ln net.Listener
		ln, err = net.Listen("tcp", tcpAddr.String())
		if err != nil {
			break
		}
		if tcpln, ok := ln.(*net.TCPListener); ok {
			tcpListeners = append(tcpListeners, tcpln)
		}
	}
	if err != nil {
		// An error occurred, cleanup.
		for _, l := range tcpListeners {
			_ = l.Close() // Ignore errors, we are already cleaning.
		}
		tcpListeners = nil
	}
	return
}

func (t TCPAddress) String() string {
	return fmt.Sprintf("%s:%d", t.Host, t.Port)
}
