// +build experimental

package libcontainerd

import (
	"fmt"

	"github.com/Sirupsen/logrus"
	containerd "github.com/docker/containerd/api/grpc/types"
)

func (clnt *client) restore(cont *containerd.Container, options ...CreateOption) (err error) {
	clnt.lock(cont.Id)
	defer clnt.unlock(cont.Id)

	logrus.Debugf("restore container %s state %s", cont.Id, cont.Status)

	containerID := cont.Id
	if _, err := clnt.getContainer(containerID); err == nil {
		return fmt.Errorf("container %s is already active", containerID)
	}

	defer func() {
		if err != nil {
			clnt.deleteContainer(cont.Id)
		}
	}()

	container := clnt.newContainer(cont.BundlePath, options...)
	container.systemPid = systemPid(cont)

	var terminal bool
	for _, p := range cont.Processes {
		if p.Pid == InitFriendlyName {
			terminal = p.Terminal
		}
	}

	iopipe, err := container.openFifos(terminal)
	if err != nil {
		return err
	}

	if err := clnt.backend.AttachStreams(containerID, *iopipe); err != nil {
		return err
	}

	clnt.appendContainer(container)

	err = clnt.backend.StateChanged(containerID, StateInfo{
		CommonStateInfo: CommonStateInfo{
			State: StateRestore,
			Pid:   container.systemPid,
		}})

	if err != nil {
		return err
	}

	if event, ok := clnt.remote.pastEvents[containerID]; ok {
		// This should only be a pause or resume event
		if event.Type == StatePause || event.Type == StateResume {
			return clnt.backend.StateChanged(containerID, StateInfo{
				CommonStateInfo: CommonStateInfo{
					State: event.Type,
					Pid:   container.systemPid,
				}})
		}

		logrus.Warnf("unexpected backlog event: %#v", event)
	}

	return nil
}

func (clnt *client) Restore(containerID string, options ...CreateOption) error {
	cont, err := clnt.getContainerdContainer(containerID)
	if err == nil && cont.Status != "stopped" {
		if err := clnt.restore(cont, options...); err != nil {
			logrus.Errorf("error restoring %s: %v", containerID, err)
		}
		return nil
	}
	return clnt.setExited(containerID)
}
