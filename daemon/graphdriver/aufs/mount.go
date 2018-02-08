// +build linux

package aufs // import "github.com/docker/docker/daemon/graphdriver/aufs"

import (
	"os/exec"

	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

// Unmount the target specified.
func Unmount(target string) error {
	if err := exec.Command("auplink", target, "flush").Run(); err != nil {
		logrus.Warnf("Couldn't run auplink before unmount %s: %s", target, err)
	}
	return unix.Unmount(target, 0)
}
