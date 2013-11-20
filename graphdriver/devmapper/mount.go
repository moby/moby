package devmapper

import (
	"os"
	"path/filepath"
)

// FIXME: this is copy-pasted from the aufs driver.
// It should be moved into the core.

func Mounted(mountpoint string) (bool, error) {
	mntpoint, err := os.Stat(mountpoint)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	parent, err := os.Stat(filepath.Join(mountpoint, ".."))
	if err != nil {
		return false, err
	}
	mntpointSt := toSysStatT(mntpoint.Sys())
	parentSt := toSysStatT(parent.Sys())
	return mntpointSt.Dev != parentSt.Dev, nil
}
