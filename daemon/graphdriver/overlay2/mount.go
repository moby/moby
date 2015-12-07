// +build linux

package overlay2

import "syscall"

// XXX: copied from AUFS
// Unmount the target specified.
func Unmount(target string) error {
	if err := syscall.Unmount(target, 0); err != nil {
		return err
	}
	return nil
}
