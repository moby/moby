package unix

import (
	"syscall"

	"golang.org/x/sys/windows"
)

func BytePtrFromString(s string) (*byte, error) {
	p, err := windows.BytePtrFromString(s)
	if err == syscall.EINVAL {
		err = EINVAL
	}
	return p, err
}

func ByteSliceToString(s []byte) string {
	return windows.ByteSliceToString(s)
}
