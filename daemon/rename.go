package daemon

import (
	"strings"

	"github.com/Sirupsen/logrus"
	derr "github.com/docker/docker/errors"
	"github.com/docker/libnetwork"
)

// ContainerRename changes the name of a container, using the oldName
// to find the container. An error is returned if newName is already
// reserved.
func (daemon *Daemon) ContainerRename(oldName, newName string) error {
	var (
		err       error
		sid       string
		sb        libnetwork.Sandbox
		container *Container
	)

	if oldName == "" || newName == "" {
		return derr.ErrorCodeEmptyRename
	}

	container, err = daemon.Get(oldName)
	if err != nil {
		return err
	}

	oldName = container.Name

	container.Lock()
	defer container.Unlock()
	if newName, err = daemon.reserveName(container.ID, newName); err != nil {
		return derr.ErrorCodeRenameTaken.WithArgs(err)
	}

	container.Name = newName

	defer func() {
		if err != nil {
			container.Name = oldName
			daemon.reserveName(container.ID, oldName)
			daemon.containerGraphDB.Delete(newName)
		}
	}()

	if err = daemon.containerGraphDB.Delete(oldName); err != nil {
		return derr.ErrorCodeRenameDelete.WithArgs(oldName, err)
	}

	if err = container.toDisk(); err != nil {
		return err
	}

	if !container.Running {
		daemon.LogContainerEvent(container, "rename")
		return nil
	}

	defer func() {
		if err != nil {
			container.Name = oldName
			if e := container.toDisk(); e != nil {
				logrus.Errorf("%s: Failed in writing to Disk on rename failure: %v", container.ID, e)
			}
		}
	}()

	sid = container.NetworkSettings.SandboxID
	sb, err = daemon.netController.SandboxByID(sid)
	if err != nil {
		return err
	}

	err = sb.Rename(strings.TrimPrefix(container.Name, "/"))
	if err != nil {
		return err
	}

	daemon.LogContainerEvent(container, "rename")
	return nil
}
