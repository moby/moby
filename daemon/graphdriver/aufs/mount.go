package aufs

import (
	"os/exec"
	"syscall"

	"github.com/docker/docker/pkg/log"
)

func Unmount(target string) error {
	if err := exec.Command("auplink", target, "flush").Run(); err != nil {
		log.Errorf("[warning]: couldn't run auplink before unmount: %s", err)
	}
	if err := syscall.Unmount(target, 0); err != nil {
		return err
	}
	return nil
}
