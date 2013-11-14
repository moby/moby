package aufs

import (
	"github.com/dotcloud/docker/utils"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

func Unmount(target string) error {
	if err := exec.Command("auplink", target, "flush").Run(); err != nil {
		utils.Errorf("[warning]: couldn't run auplink before unmount: %s", err)
	}
	if err := syscall.Unmount(target, 0); err != nil {
		return err
	}
	return nil
}

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
	mntpointSt := mntpoint.Sys().(*syscall.Stat_t)
	parentSt := parent.Sys().(*syscall.Stat_t)

	return mntpointSt.Dev != parentSt.Dev, nil
}
