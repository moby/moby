//go:build !linux && !darwin && !freebsd && !netbsd

package archive

// lgetxattr is not supported on Windows.
func lgetxattr(path string, attr string) ([]byte, error) {
	return nil, nil
}

// lsetxattr is not supported on Windows.
func lsetxattr(path string, attr string, data []byte, flags int) error {
	return nil
}
