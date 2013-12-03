// +build darwin

package docker

func btrfs_reflink(fd_out, fd_in uintptr) int {
	return 255
}
