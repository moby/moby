//go:build !windows

// Package unix_noeintr provides wrappers for unix syscalls that retry on EINTR.
//
// TODO: Consider moving (for example to moby/sys) and making the wrappers auto-generated.
package unix_noeintr

import (
	"errors"

	"golang.org/x/sys/unix"
)

func Retry(f func() error) {
	for {
		err := f()
		if !errors.Is(err, unix.EINTR) {
			return
		}
	}
}

func Mount(source string, target string, fstype string, flags uintptr, data string) (err error) {
	Retry(func() error {
		err = unix.Mount(source, target, fstype, flags, data)
		return err
	})
	return err
}

func Unmount(target string, flags int) (err error) {
	Retry(func() error {
		err = unix.Unmount(target, flags)
		return err
	})
	return err
}

func Open(path string, mode int, perm uint32) (fd int, err error) {
	Retry(func() error {
		fd, err = unix.Open(path, mode, perm)
		return err
	})
	return fd, err
}

func Close(fd int) (err error) {
	Retry(func() error {
		err = unix.Close(fd)
		return err
	})
	return err
}

func Openat(dirfd int, path string, mode int, perms uint32) (fd int, err error) {
	Retry(func() error {
		fd, err = unix.Openat(dirfd, path, mode, perms)
		return err
	})
	return fd, err
}

func Openat2(dirfd int, path string, how *unix.OpenHow) (fd int, err error) {
	Retry(func() error {
		fd, err = unix.Openat2(dirfd, path, how)
		return err
	})
	return fd, err
}

func Fstat(fd int, stat *unix.Stat_t) (err error) {
	Retry(func() error {
		err = unix.Fstat(fd, stat)
		return err
	})
	return err
}

func Fstatat(fd int, path string, stat *unix.Stat_t, flags int) (err error) {
	Retry(func() error {
		err = unix.Fstatat(fd, path, stat, flags)
		return err
	})
	return err
}
