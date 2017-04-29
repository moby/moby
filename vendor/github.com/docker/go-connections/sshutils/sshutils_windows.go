// +build windows

package sshutils

import (
	"errors"
	"os"
)

func getHome() (string, error) {
	// TODO: use docker/pkg/homedir (mess for static bin, see comments in homedir_linux.go)
	home := os.Getenv("USERPROFILE")
	if home == "" {
		return "", errors.New("USERPROFILE unset")
	}
	return home, nil
}
