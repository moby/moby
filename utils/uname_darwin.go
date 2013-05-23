package utils

import (
	"errors"
)

type Utsname struct {
	Release [65]byte
}

func uname() (*Utsname, error) {
	return nil, errors.New("Kernel version detection is not available on darwin")
}
