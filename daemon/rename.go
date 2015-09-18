package daemon

import (
	derr "github.com/docker/docker/errors"
)

// ContainerRename changes the name of a container, using the oldName
// to find the container. An error is returned if newName is already
// reserved.
func (daemon *Daemon) ContainerRename(oldName, newName string) error {
	if oldName == "" || newName == "" {
		return derr.ErrorCodeEmptyRename
	}

	container, err := daemon.Get(oldName)
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

	undo := func() {
		container.Name = oldName
		daemon.reserveName(container.ID, oldName)
		daemon.containerGraphDB.Delete(newName)
	}

	if err := daemon.containerGraphDB.Delete(oldName); err != nil {
		undo()
		return derr.ErrorCodeRenameDelete.WithArgs(oldName, err)
	}

	if err := container.toDisk(); err != nil {
		undo()
		return err
	}

	container.logEvent("rename")
	return nil
}
