package aufs

import (
	"github.com/dotcloud/docker/utils"
	"os/exec"
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
