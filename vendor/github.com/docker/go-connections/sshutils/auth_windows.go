// +build windows

package sshutils

import (
	"golang.org/x/crypto/ssh"

	"github.com/Sirupsen/logrus"
	"github.com/davidmz/go-pageant"
)

func resolveAgent(c *config) ssh.AuthMethod {
	if pageant.Available() {
		logrus.Debugf("detected pageant window")
		return ssh.PublicKeysCallback(pageant.New().Signers)
	}
	return nil
}
