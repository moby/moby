// +build !windows

package sshutils

import (
	"net"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"

	"github.com/Sirupsen/logrus"
)

func resolveAgent(c *config) ssh.AuthMethod {
	if sshAgent, err := net.Dial("unix", c.sshAuthSock); err == nil {
		logrus.Debugf("detected ssh agent socket")
		return ssh.PublicKeysCallback(agent.NewClient(sshAgent).Signers)
	}
	return nil
}
