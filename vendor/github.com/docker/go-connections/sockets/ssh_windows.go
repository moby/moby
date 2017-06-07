// +build windows

package sockets

import (
	"errors"
	"net"

	"github.com/davidmz/go-pageant"
	"golang.org/x/crypto/ssh"
)

func dialSSH(user, addr, socketPath string) (net.Conn, error) {
	if !pageant.Available() {
		return nil, errors.New("pageant not running")
	}
	auth := ssh.PublicKeysCallback(pageant.New().Signers)
	sshClient, err := ssh.Dial("tcp", addr, &ssh.ClientConfig{
		User:            user,
		Auth:            []ssh.AuthMethod{auth},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	})
	if err != nil {
		return nil, err
	}
	return sshClient.Dial("unix", socketPath)
}
