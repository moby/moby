// +build !windows

package system

import (
	"syscall"
)

func Lstat(path string) (*syscall.Stat_t, error) {
	s := &syscall.Stat_t{}
	err := syscall.Lstat(path, s)
	if err != nil {
		return nil, err
	}
	return s, nil
}
