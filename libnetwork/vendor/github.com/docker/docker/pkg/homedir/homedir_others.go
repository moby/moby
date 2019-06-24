// +build !linux

package homedir // import "github.com/docker/docker/pkg/homedir"

import (
	"errors"
)

// GetStatic is not needed for non-linux systems.
// (Precisely, it is needed only for glibc-based linux systems.)
func GetStatic() (string, error) {
	return "", errors.New("homedir.GetStatic() is not supported on this system")
}

// GetRuntimeDir is unsupported on non-linux system.
func GetRuntimeDir() (string, error) {
	return "", errors.New("homedir.GetRuntimeDir() is not supported on this system")
}

// StickRuntimeDirContents is unsupported on non-linux system.
func StickRuntimeDirContents(files []string) ([]string, error) {
	return nil, errors.New("homedir.StickRuntimeDirContents() is not supported on this system")
}

// GetDataHome is unsupported on non-linux system.
func GetDataHome() (string, error) {
	return "", errors.New("homedir.GetDataHome() is not supported on this system")
}

// GetConfigHome is unsupported on non-linux system.
func GetConfigHome() (string, error) {
	return "", errors.New("homedir.GetConfigHome() is not supported on this system")
}
