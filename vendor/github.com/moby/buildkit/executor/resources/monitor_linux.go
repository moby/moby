//go:build linux

package resources

import "golang.org/x/sys/unix"

func isCgroup2() bool {
	var st unix.Statfs_t
	err := unix.Statfs(defaultMountpoint, &st)
	if err != nil {
		return false
	}
	return st.Type == unix.CGROUP2_SUPER_MAGIC
}
