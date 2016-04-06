package graphdriver

import "syscall"

var (
	// Slice of drivers that should be used in an order
	priority = []string{
		"zfs",
	}
)

// Mounted checks if the given path is mounted as the fs type
func Mounted(fsType FsMagic, mountPath string) (bool, error) {
	var buf syscall.Statfs_t
	if err := syscall.Statfs(mountPath, &buf); err != nil {
		return false, err
	}
	return FsMagic(buf.Type) == fsType, nil
}
