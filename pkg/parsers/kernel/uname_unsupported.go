//go:build !linux

package kernel

import (
	"errors"
)

// utsName represents the system name structure. It is defined here to make it
// portable as it is available on Linux but not on Windows.
type utsName struct {
	Release [65]byte
}

func uname() (*utsName, error) {
	return nil, errors.New("kernel version detection is only available on linux")
}
