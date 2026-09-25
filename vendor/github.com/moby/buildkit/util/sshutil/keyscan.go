package sshutil

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"

	"golang.org/x/crypto/ssh"
)

const defaultPort = 22

var errCallbackDone = errors.New("callback failed on purpose")

// knownHostsServerID formats a host identifier for a known_hosts entry. A
// non-standard port must be rendered as "[host]:port" so that ssh matches the
// entry when connecting (see the SSH_KNOWN_HOSTS format in sshd(8)); the
// default port is rendered as a bare hostname.
func knownHostsServerID(hostname, port string) string {
	if port == "" || port == strconv.Itoa(defaultPort) {
		return hostname
	}
	return fmt.Sprintf("[%s]:%s", hostname, port)
}

// addDefaultPort appends a default port if hostport doesn't contain one
func addDefaultPort(hostport string, defaultPort int) string {
	_, _, err := net.SplitHostPort(hostport)
	if err == nil {
		return hostport
	}
	hostport = net.JoinHostPort(hostport, strconv.Itoa(defaultPort))
	return hostport
}

// SSHKeyScan scans a ssh server for the hostkey; server should be in the form hostname, or hostname:port
func SSHKeyScan(server string) (string, error) {
	var key string
	KeyScanCallback := func(hostport string, remote net.Addr, pubKey ssh.PublicKey) error {
		hostname, port, err := net.SplitHostPort(hostport)
		if err != nil {
			return err
		}
		serverID := knownHostsServerID(hostname, port)
		key = strings.TrimSpace(fmt.Sprintf("%s %s", serverID, string(ssh.MarshalAuthorizedKey(pubKey))))
		return errCallbackDone
	}
	config := &ssh.ClientConfig{
		HostKeyCallback: KeyScanCallback,
	}

	server = addDefaultPort(server, defaultPort)
	conn, err := ssh.Dial("tcp", server, config)
	if key != "" {
		// as long as we get the key, the function worked
		err = nil
	}
	if conn != nil {
		_ = conn.Close()
	}
	return key, err
}
