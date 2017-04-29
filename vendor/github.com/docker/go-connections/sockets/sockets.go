// Package sockets provides helper functions to create and configure Unix or TCP sockets.
package sockets

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/docker/go-connections/sshutils"
	"golang.org/x/crypto/ssh"
)

// Why 32? See https://github.com/docker/docker/pull/8035.
const defaultTimeout = 32 * time.Second

// ErrProtocolNotAvailable is returned when a given transport protocol is not provided by the operating system.
var ErrProtocolNotAvailable = errors.New("protocol not available")

// ConfigureTransport configures the specified Transport according to the
// specified proto and addr.
// If the proto is unix (using a unix socket to communicate) or npipe the
// compression is disabled.
func ConfigureTransport(tr *http.Transport, proto, addr string) error {
	switch proto {
	case "unix":
		return configureUnixTransport(tr, proto, addr)
	case "npipe":
		return configureNpipeTransport(tr, proto, addr)
	case "ssh": // unix over ssh
		return configureSSHTransport(tr, proto, addr)
	default:
		tr.Proxy = http.ProxyFromEnvironment
		dialer, err := DialerFromEnvironment(&net.Dialer{
			Timeout: defaultTimeout,
		})
		if err != nil {
			return err
		}
		tr.Dial = dialer.Dial
	}
	return nil
}

func configureSSHTransport(tr *http.Transport, proto, addr string) error {
	_, dialer, err := prepareSSH(addr)
	if err != nil {
		return err
	}
	tr.Dial = func(_, _ string) (net.Conn, error) {
		return dialer()
	}
	return nil
}

// DialSSH connects to a Unix socket over SSH.
func DialSSH(addr string) (net.Conn, error) {
	_, dialer, err := prepareSSH(addr)
	if err != nil {
		return nil, err
	}
	return dialer()
}

func prepareSSH(addr string) (*ssh.Client, func() (net.Conn, error), error) {
	u, err := url.Parse("ssh://" + addr)
	if err != nil {
		return nil, nil, err
	}
	if u.User == nil || u.User.Username() == "" {
		return nil, nil, fmt.Errorf("ssh requires username")
	}
	if _, ok := u.User.Password(); ok {
		return nil, nil, fmt.Errorf("ssh does not accept plain-text password")
	}
	if u.Path == "" {
		return nil, nil, fmt.Errorf("ssh requires socket path")
	}
	sshClient, err := sshutils.Dial(u.User.Username(), "tcp", u.Host)
	if err != nil {
		return nil, nil, err
	}
	dialer := func() (net.Conn, error) {
		return sshClient.Dial("unix", u.Path)
	}
	return sshClient, dialer, nil
}
