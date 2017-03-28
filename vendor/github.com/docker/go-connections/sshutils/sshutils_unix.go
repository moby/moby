// +build !windows

package sshutils

import (
	"errors"
	"os"
)

func getHome() (string, error) {
	// TODO: use docker/pkg/homedir (mess for static bin, see comments in homedir_linux.go)
	home := os.Getenv("HOME")
	if home == "" {
		return "", errors.New("HOME unset")
	}
	return home, nil
}
