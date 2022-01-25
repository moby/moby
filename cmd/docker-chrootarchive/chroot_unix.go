//go:build !windows && !linux && !cgo
// +build !windows,!linux,!cgo

package main

import "golang.org/x/sys/unix"

func chroot(path string) error {
	if err := unix.Chroot(path); err != nil {
		return err
	}
	return unix.Chdir("/")
}

func realChroot(path string) error {
	return chroot(path)
}
