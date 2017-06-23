package daemon

import (
	"errors"
	"fmt"
	"strings"

	"github.com/Sirupsen/logrus"
	dockercontainer "github.com/docker/docker/container"
	"github.com/docker/libnetwork"
)

// ContainerRename changes the name of a container, using the oldName
// to find the container. An error is returned if newName is already
// reserved.
func (daemon *Daemon) ContainerRename(oldName, newName string) error {
	var (
		sid string
		sb  libnetwork.Sandbox
	)

	if oldName == "" || newName == "" {
		return errors.New("Neither old nor new names may be empty")
	}

	if newName[0] != '/' {
		newName = "/" + newName
	}

	container, err := daemon.GetContainer(oldName)
	if err != nil {
		return err
	}

	container.Lock()
	defer container.Unlock()

	oldName = container.Name
	oldIsAnonymousEndpoint := container.NetworkSettings.IsAnonymousEndpoint

	if oldName == newName {
		return errors.New("Renaming a container with the same name as its current name")
	}

	links := map[string]*dockercontainer.Container{}
	for k, v := range daemon.linkIndex.children(container) {
		if !strings.HasPrefix(k, oldName) {
			return fmt.Errorf("Linked container %s does not match parent %s", k, oldName)
		}
		links[strings.TrimPrefix(k, oldName)] = v
	}

	if newName, err = daemon.reserveName(container.ID, newName); err != nil {
		return fmt.Errorf("Error when allocating new name: %v", err)
	}

	for k, v := range links {
		daemon.nameIndex.Reserve(newName+k, v.ID)
		daemon.linkIndex.link(container, v, newName+k)
	}

	container.Name = newName
	container.NetworkSettings.IsAnonymousEndpoint = false

	defer func() {
		if err != nil {
			container.Name = oldName
			container.NetworkSettings.IsAnonymousEndpoint = oldIsAnonymousEndpoint
			daemon.reserveName(container.ID, oldName)
			for k, v := range links {
				daemon.nameIndex.Reserve(oldName+k, v.ID)
				daemon.linkIndex.link(container, v, oldName+k)
				daemon.linkIndex.unlink(newName+k, v, container)
				daemon.nameIndex.Release(newName + k)
			}
			daemon.releaseName(newName)
		}
	}()

	for k, v := range links {
		daemon.linkIndex.unlink(oldName+k, v, container)
		daemon.nameIndex.Release(oldName + k)
	}
	daemon.releaseName(oldName)
	if err = container.CheckpointTo(daemon.containersReplica); err != nil {
		return err
	}

	attributes := map[string]string{
		"oldName": oldName,
	}

	if !container.Running {
		daemon.LogContainerEventWithAttributes(container, "rename", attributes)
		return nil
	}

	defer func() {
		if err != nil {
			container.Name = oldName
			container.NetworkSettings.IsAnonymousEndpoint = oldIsAnonymousEndpoint
			if e := container.CheckpointTo(daemon.containersReplica); e != nil {
				logrus.Errorf("%s: Failed in writing to Disk on rename failure: %v", container.ID, e)
			}
		}
	}()

	sid = container.NetworkSettings.SandboxID
	if sid != "" && daemon.netController != nil {
		sb, err = daemon.netController.SandboxByID(sid)
		if err != nil {
			return err
		}

		err = sb.Rename(strings.TrimPrefix(container.Name, "/"))
		if err != nil {
			return err
		}
	}

	daemon.LogContainerEventWithAttributes(container, "rename", attributes)
	return nil
}

// Volume rename renames a given volume.
// If the volume is referenced by a running container it is not renamed.
// This is called directly from the Engine API
func (daemon *Daemon) VolumeRename(name string, newName string) error {
	var runningContainers []string

	v, err := daemon.volumes.Get(name)
	if err != nil {
		return err
	}

	if daemon.volumes.HasRef(name) {
		refs := daemon.volumes.GetRefs(name)
		for _, containerID := range refs {
			container, err := daemon.GetContainer(containerID)
			if err != nil {
				return err
			}

			if container.IsRunning() {
				runningContainers = append(runningContainers, containerID)
			}
		}
	}

	if len(runningContainers) > 0 {
		return fmt.Errorf("Volume %s is in use by running containers %v. Stop the containers before renaming the volume.\n", v.Name(), runningContainers)
	}

	if err := daemon.volumes.Rename(v, newName); err != nil {
		return err
	}

	// Update Container config for stopped containers.
	// Update config.v2.json and hostconfig.json where there are references
	// of old volume name. This solution is very hacky, so open a discussion upstream.
	return nil
}
