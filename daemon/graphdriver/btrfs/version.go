// +build linux

package btrfs

/*
#include <btrfs/version.h>
*/
import "C"

func BtrfsBuildVersion() string {
	return string(C.BTRFS_BUILD_VERSION)
}
func BtrfsLibVersion() int {
	return int(C.BTRFS_LIB_VERSION)
}
