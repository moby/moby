package sshutil

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	"golang.org/x/crypto/ssh"
)

const defaultPort = 22

var errCallbackDone = fmt.Errorf("callback failed on purpose")

// addDefaultPort appends a default port if hostport doesn't contain one
func addDefaultPort(hostport string, defaultPort int) string {
	_, _, err := net.SplitHostPort(hostport)
	if err == nil {
		return hostport
	}
	hostport = net.JoinHostPort(hostport, strconv.Itoa(defaultPort))
	return hostport
}

// SshKeyScan scans a ssh server for the hostkey; server should be in the form hostname, or hostname:port
func SSHKeyScan(server string) (string, error) {
	var key string
	KeyScanCallback := func(hostport string, remote net.Addr, pubKey ssh.PublicKey) error {
		hostname, _, err := net.SplitHostPort(hostport)
		if err != nil {
			return err
		}
		key = strings.TrimSpace(fmt.Sprintf("%s %s", hostname, string(ssh.MarshalAuthorizedKey(pubKey))))
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
		conn.Close()
	}
	return key, err
}
