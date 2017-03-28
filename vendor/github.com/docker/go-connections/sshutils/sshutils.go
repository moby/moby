// Package sshutils provides utilities for SSH
package sshutils

import (
	"errors"
	"io/ioutil"
	"os"

	"golang.org/x/crypto/ssh"
)

// ResolveAuthMethods returns the list of available auth methods.
// Currently supported methods:
//  - ssh-agent (on Unixen)
//  - pageant   (on Windows)
//  - publickey (files under $HOME/.ssh)
//
// Methods unlikely to be supported in future:
//  - keyboard interactive
//
// NOTE: currently, ~/.ssh/config and /etc/ssh/ssh_config are unused.
func ResolveAuthMethods() ([]ssh.AuthMethod, error) {
	home, err := getHome()
	if err != nil {
		return nil, err
	}
	return resolveAuthMethods(home, os.Getenv, ioutil.ReadFile)
}

// Dial dials with available auth methods
func Dial(user, n, addr string) (*ssh.Client, error) {
	auths, err := ResolveAuthMethods()
	if err != nil {
		return nil, err
	}
	if len(auths) == 0 {
		return nil, errors.New("no auth method found")
	}
	return ssh.Dial(n, addr, &ssh.ClientConfig{
		User: user,
		Auth: auths,
	})
}
