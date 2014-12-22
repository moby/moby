// +build linux,!btrfs_noversion

package btrfs

/*
#include <btrfs/version.h>

// because around version 3.16, they did not define lib version yet
int my_btrfs_lib_version() {
#ifdef BTRFS_LIB_VERSION
  return BTRFS_LIB_VERSION;
#else
  return -1;
#endif
}
*/
import "C"

func BtrfsBuildVersion() string {
	return string(C.BTRFS_BUILD_VERSION)
}
func BtrfsLibVersion() int {
	return int(C.BTRFS_LIB_VERSION)
}
