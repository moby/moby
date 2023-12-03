package daemon // import "github.com/docker/docker/daemon"

import (
	"context"
	"strings"

	"github.com/containerd/log"
	"github.com/pkg/errors"

	"github.com/docker/docker/api/types/events"
	dockercontainer "github.com/docker/docker/container"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/libnetwork"
)

// ContainerRename changes the name of a container, using the oldName
// to find the container. An error is returned if newName is already
// reserved.
func (daemon *Daemon) ContainerRename(oldName, newName string) (retErr error) {
	var (
		sid string
		sb  *libnetwork.Sandbox
	)

	if oldName == "" || newName == "" {
		return errdefs.InvalidParameter(errors.New("Neither old nor new names may be empty"))
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
		return errdefs.InvalidParameter(errors.New("Renaming a container with the same name as its current name"))
	}

	links := map[string]*dockercontainer.Container{}
	for k, v := range daemon.linkIndex.children(container) {
		if !strings.HasPrefix(k, oldName) {
			return errdefs.InvalidParameter(errors.Errorf("Linked container %s does not match parent %s", k, oldName))
		}
		links[strings.TrimPrefix(k, oldName)] = v
	}

	if newName, err = daemon.reserveName(container.ID, newName); err != nil {
		return errors.Wrap(err, "Error when allocating new name")
	}

	for k, v := range links {
		daemon.containersReplica.ReserveName(newName+k, v.ID)
		daemon.linkIndex.link(container, v, newName+k)
	}

	container.Name = newName
	container.NetworkSettings.IsAnonymousEndpoint = false

	defer func() {
		if retErr != nil {
			container.Name = oldName
			container.NetworkSettings.IsAnonymousEndpoint = oldIsAnonymousEndpoint
			daemon.reserveName(container.ID, oldName)
			for k, v := range links {
				daemon.containersReplica.ReserveName(oldName+k, v.ID)
				daemon.linkIndex.link(container, v, oldName+k)
				daemon.linkIndex.unlink(newName+k, v, container)
				daemon.containersReplica.ReleaseName(newName + k)
			}
			daemon.releaseName(newName)
		} else {
			daemon.releaseName(oldName)
		}
	}()

	for k, v := range links {
		daemon.linkIndex.unlink(oldName+k, v, container)
		daemon.containersReplica.ReleaseName(oldName + k)
	}
	if err = container.CheckpointTo(daemon.containersReplica); err != nil {
		return err
	}

	attributes := map[string]string{
		"oldName": oldName,
	}

	if !container.Running {
		daemon.LogContainerEventWithAttributes(container, events.ActionRename, attributes)
		return nil
	}

	defer func() {
		if retErr != nil {
			container.Name = oldName
			container.NetworkSettings.IsAnonymousEndpoint = oldIsAnonymousEndpoint
			if err := container.CheckpointTo(daemon.containersReplica); err != nil {
				log.G(context.TODO()).WithFields(log.Fields{
					"containerID": container.ID,
					"error":       err,
				}).Error("failed to write container state to disk during rename")
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

	daemon.LogContainerEventWithAttributes(container, events.ActionRename, attributes)
	return nil
}
