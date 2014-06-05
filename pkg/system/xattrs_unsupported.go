// +build !linux

package system

func Llistxattr(path string, dest []byte) (size int, err error) {
	return -1, ErrNotSupportedPlatform
}

func Lgetxattr(path string, attr string) ([]byte, error) {
	return nil, ErrNotSupportedPlatform
}

func Lsetxattr(path string, attr string, data []byte, flags int) error {
	return ErrNotSupportedPlatform
}
