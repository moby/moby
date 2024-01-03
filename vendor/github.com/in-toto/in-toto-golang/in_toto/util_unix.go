//go:build linux || darwin || !windows
// +build linux darwin !windows

package in_toto

import "golang.org/x/sys/unix"

func isWritable(path string) error {
	err := unix.Access(path, unix.W_OK)
	if err != nil {
		return err
	}
	return nil
}
