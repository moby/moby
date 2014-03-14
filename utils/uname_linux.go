// +build amd64

package utils

import (
	"syscall"
)

type Utsname syscall.Utsname

func uname() (*syscall.Utsname, error) {
	uts := &syscall.Utsname{}

	if err := syscall.Uname(uts); err != nil {
		return nil, err
	}
	return uts, nil
}
