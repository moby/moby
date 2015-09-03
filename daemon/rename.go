package daemon

import (
	"fmt"
)

// ContainerRename changes the name of a container.
// An error is returned if newName is already reserved.
func (daemon *Daemon) ContainerRename(container *Container, newName string) error {
	if newName == "" {
		return fmt.Errorf("usage: docker rename OLD_NAME NEW_NAME")
	}

	oldName := container.Name

	container.Lock()
	defer container.Unlock()
	newName, err := daemon.reserveName(container.ID, newName)
	if err != nil {
		return fmt.Errorf("Error when allocating new name: %s", err)
	}

	container.Name = newName

	undo := func() {
		container.Name = oldName
		daemon.reserveName(container.ID, oldName)
		daemon.containerGraphDB.Delete(newName)
	}

	if err := daemon.containerGraphDB.Delete(oldName); err != nil {
		undo()
		return fmt.Errorf("Failed to delete container %q: %v", oldName, err)
	}

	if err := container.toDisk(); err != nil {
		undo()
		return err
	}

	container.logEvent("rename")
	return nil
}
