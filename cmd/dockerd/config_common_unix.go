//go:build linux || freebsd
// +build linux freebsd

package main

import (
	"path/filepath"

	"github.com/docker/docker/pkg/homedir"
)

func getDefaultPidFile() (string, error) {
	if !honorXDG {
		return "/var/run/docker.pid", nil
	}
	runtimeDir, err := homedir.GetRuntimeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(runtimeDir, "docker.pid"), nil
}

func getDefaultDataRoot() (string, error) {
	if !honorXDG {
		return "/var/lib/docker", nil
	}
	dataHome, err := homedir.GetDataHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(dataHome, "docker"), nil
}

func getDefaultExecRoot() (string, error) {
	if !honorXDG {
		return "/var/run/docker", nil
	}
	runtimeDir, err := homedir.GetRuntimeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(runtimeDir, "docker"), nil
}
