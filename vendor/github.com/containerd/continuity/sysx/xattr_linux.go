package sysx

import "golang.org/x/sys/unix"

// Listxattr calls syscall listxattr and reads all content
// and returns a string array
func Listxattr(path string) ([]string, error) {
	return listxattrAll(path, unix.Listxattr)
}

// Removexattr calls syscall removexattr
func Removexattr(path string, attr string) (err error) {
	return unix.Removexattr(path, attr)
}

// Setxattr calls syscall setxattr
func Setxattr(path string, attr string, data []byte, flags int) (err error) {
	return unix.Setxattr(path, attr, data, flags)
}

// Getxattr calls syscall getxattr
func Getxattr(path, attr string) ([]byte, error) {
	return getxattrAll(path, attr, unix.Getxattr)
}

// LListxattr lists xattrs, not following symlinks
func LListxattr(path string) ([]string, error) {
	return listxattrAll(path, unix.Llistxattr)
}

// LRemovexattr removes an xattr, not following symlinks
func LRemovexattr(path string, attr string) (err error) {
	return unix.Lremovexattr(path, attr)
}

// LSetxattr sets an xattr, not following symlinks
func LSetxattr(path string, attr string, data []byte, flags int) (err error) {
	return unix.Lsetxattr(path, attr, data, flags)
}

// LGetxattr gets an xattr, not following symlinks
func LGetxattr(path, attr string) ([]byte, error) {
	return getxattrAll(path, attr, unix.Lgetxattr)
}
