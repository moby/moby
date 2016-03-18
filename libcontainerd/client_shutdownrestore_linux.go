// +build !experimental

package libcontainerd

import (
	"syscall"
	"time"

	"github.com/Sirupsen/logrus"
)

func (clnt *client) Restore(containerID string, options ...CreateOption) error {
	w := clnt.getOrCreateExitNotifier(containerID)
	defer w.close()
	cont, err := clnt.getContainerdContainer(containerID)
	if err == nil && cont.Status != "stopped" {
		clnt.lock(cont.Id)
		container := clnt.newContainer(cont.BundlePath)
		container.systemPid = systemPid(cont)
		clnt.appendContainer(container)
		clnt.unlock(cont.Id)

		if err := clnt.Signal(containerID, int(syscall.SIGTERM)); err != nil {
			logrus.Errorf("error sending sigterm to %v: %v", containerID, err)
		}
		select {
		case <-time.After(10 * time.Second):
			if err := clnt.Signal(containerID, int(syscall.SIGKILL)); err != nil {
				logrus.Errorf("error sending sigkill to %v: %v", containerID, err)
			}
			select {
			case <-time.After(2 * time.Second):
			case <-w.wait():
			}
		case <-w.wait():
		}
	}
	return clnt.setExited(containerID)
}
