//go:build linux || darwin || freebsd || netbsd

package fsutil

import (
	"os"
	"syscall"

	"github.com/pkg/errors"
	"github.com/tonistiigi/fsutil/types"
	"golang.org/x/sys/unix"
)

const rootXattrBufferSize = 128

func loadRootXattr(root Root, path string, stat *types.Stat) error {
	f, err := root.OpenFile(path, os.O_RDONLY|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0)
	if err != nil {
		if ignoreRootXattrOpenError(err) {
			return nil
		}
		return errors.WithStack(err)
	}
	defer f.Close()

	xattrs, err := rootListxattr(int(f.Fd()))
	if err != nil {
		if errors.Is(err, syscall.ENOTSUP) || errors.Is(err, unix.EOPNOTSUPP) || errors.Is(err, unix.ENOSYS) {
			return nil
		}
		return errors.Wrapf(err, "failed to xattr %s", path)
	}
	if len(xattrs) == 0 {
		return nil
	}

	m := make(map[string][]byte)
	for _, key := range xattrs {
		if skipXattr(key) {
			continue
		}
		if v, err := rootGetxattr(int(f.Fd()), key); err == nil {
			m[key] = v
		}
	}
	if len(m) > 0 {
		stat.Xattrs = m
	}
	return nil
}

func ignoreRootXattrOpenError(err error) bool {
	return errors.Is(err, syscall.ELOOP) ||
		errors.Is(err, syscall.EACCES) ||
		errors.Is(err, syscall.EPERM) ||
		errors.Is(err, syscall.ENXIO)
}

func rootListxattr(fd int) ([]string, error) {
	return rootListxattrWith(fd, rootFlistxattr, rootParseListxattr)
}

func rootGetxattr(fd int, key string) ([]byte, error) {
	buf := make([]byte, rootXattrBufferSize)
	n, err := unix.Fgetxattr(fd, key, buf)
	for err == unix.ERANGE {
		n, err = unix.Fgetxattr(fd, key, nil)
		if err != nil {
			return nil, err
		}
		buf = make([]byte, n)
		n, err = unix.Fgetxattr(fd, key, buf)
	}
	if err != nil {
		return nil, err
	}
	return buf[:n], nil
}

type rootListxattrFunc func(int, []byte) (int, error)
type rootParseListxattrFunc func([]byte) []string

func rootListxattrWith(fd int, list rootListxattrFunc, parse rootParseListxattrFunc) ([]string, error) {
	buf := make([]byte, rootXattrBufferSize)
	n, err := list(fd, buf)
	for err == unix.ERANGE {
		n, err = list(fd, nil)
		if err != nil {
			return nil, err
		}
		buf = make([]byte, n)
		n, err = list(fd, buf)
	}
	if err != nil {
		return nil, err
	}
	return parse(buf[:n]), nil
}
