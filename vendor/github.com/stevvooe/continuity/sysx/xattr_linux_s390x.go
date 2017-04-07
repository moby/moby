package sysx

import "github.com/pkg/errors"

func llistxattr(path string, dest []byte) (sz int, err error) {
	return 0, errors.Wrap(ErrNotSupported, "llistxattr not implemented on s390x")
}

func lremovexattr(path string, attr string) (err error) {
	return errors.Wrap(ErrNotSupported, "lremovexattr not implemented on s390x")
}

func lsetxattr(path string, attr string, data []byte, flags int) (err error) {
	return errors.Wrap(ErrNotSupported, "lsetxattr not implemented on s390x")
}

func lgetxattr(path string, attr string, dest []byte) (sz int, err error) {
	return 0, errors.Wrap(ErrNotSupported, "lgetxattr not implemented on s390x")
}
