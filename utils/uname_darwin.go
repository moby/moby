package utils

import (
	"errors"
	"syscall"
)

func uname() (*syscall.Utsname, error) {
	return nil, errors.New("Kernel version detection is not available on darwin")
}
