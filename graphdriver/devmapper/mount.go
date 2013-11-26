package devmapper

import (
	"path/filepath"
)

// FIXME: this is copy-pasted from the aufs driver.
// It should be moved into the core.

var Mounted = func(mountpoint string) (bool, error) {
	mntpoint, err := osStat(mountpoint)
	if err != nil {
		if osIsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	parent, err := osStat(filepath.Join(mountpoint, ".."))
	if err != nil {
		return false, err
	}
	mntpointSt := toSysStatT(mntpoint.Sys())
	parentSt := toSysStatT(parent.Sys())
	return mntpointSt.Dev != parentSt.Dev, nil
}
