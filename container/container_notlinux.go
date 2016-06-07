// +build solaris freebsd

package container

import (
	"golang.org/x/sys/unix"
)

func detachMounted(path string) error {
	//Solaris and FreeBSD do not support the lazy unmount or MNT_DETACH feature.
	// Therefore there are separate definitions for this.
	return unix.Unmount(path, 0)
}
