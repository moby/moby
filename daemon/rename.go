package daemon // import "github.com/docker/docker/daemon"

import (
	"context"
	"strings"

	"github.com/containerd/log"
	"github.com/docker/docker/api/types/events"
	dockercontainer "github.com/docker/docker/container"
	"github.com/docker/docker/daemon/network"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/libnetwork"
	"github.com/pkg/errors"
)

// ContainerRename changes the name of a container, using the oldName
// to find the container. An error is returned if newName is already
// reserved.
func (daemon *Daemon) ContainerRename(oldName, newName string) (retErr error) {
	if oldName == "" || newName == "" {
		return errdefs.InvalidParameter(errors.New("Neither old nor new names may be empty"))
	}

	container, err := daemon.GetContainer(oldName)
	if err != nil {
		return err
	}
	container.Lock()
	defer container.Unlock()

	// Canonicalize name for comparing.
	if newName[0] != '/' {
		newName = "/" + newName
	}
	if container.Name == newName {
		return errdefs.InvalidParameter(errors.New("Renaming a container with the same name as its current name"))
	}

	links := map[string]*dockercontainer.Container{}
	for k, v := range daemon.linkIndex.children(container) {
		if !strings.HasPrefix(k, container.Name) {
			return errdefs.InvalidParameter(errors.Errorf("Linked container %s does not match parent %s", k, container.Name))
		}
		links[strings.TrimPrefix(k, container.Name)] = v
	}

	newName, err = daemon.reserveName(container.ID, newName)
	if err != nil {
		return errors.Wrap(err, "Error when allocating new name")
	}

	for k, v := range links {
		daemon.containersReplica.ReserveName(newName+k, v.ID)
		daemon.linkIndex.link(container, v, newName+k)
	}

	oldName = container.Name
	container.Name = newName

	defer func() {
		if retErr != nil {
			container.Name = oldName
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
	if err := container.CheckpointTo(daemon.containersReplica); err != nil {
		return err
	}

	if !container.Running {
		daemon.LogContainerEventWithAttributes(container, events.ActionRename, map[string]string{
			"oldName": oldName,
		})
		return nil
	}

	defer func() {
		if retErr != nil {
			container.Name = oldName
			if err := container.CheckpointTo(daemon.containersReplica); err != nil {
				log.G(context.TODO()).WithFields(log.Fields{
					"containerID": container.ID,
					"error":       err,
				}).Error("failed to write container state to disk during rename")
			}
		}
	}()

	if sid := container.NetworkSettings.SandboxID; sid != "" && daemon.netController != nil {
		sb, err := daemon.netController.SandboxByID(sid)
		if err != nil {
			return err
		}
		if err = sb.Rename(strings.TrimPrefix(container.Name, "/")); err != nil {
			return err
		}
		defer func() {
			if retErr != nil {
				if err := sb.Rename(oldName); err != nil {
					log.G(context.TODO()).WithFields(log.Fields{
						"sandboxID": sid,
						"oldName":   oldName,
						"newName":   newName,
						"error":     err,
					}).Errorf("failed to revert sandbox rename")
				}
			}
		}()

		for nwName, epConfig := range container.NetworkSettings.Networks {
			nw, err := daemon.FindNetwork(nwName)
			if err != nil {
				return err
			}

			ep, err := nw.EndpointByID(epConfig.EndpointID)
			if err != nil {
				return err
			}

			oldDNSNames := make([]string, len(epConfig.DNSNames))
			copy(oldDNSNames, epConfig.DNSNames)

			epConfig.DNSNames = buildEndpointDNSNames(container, epConfig.Aliases)
			if err := ep.UpdateDNSNames(epConfig.DNSNames); err != nil {
				return err
			}

			defer func(ep *libnetwork.Endpoint, epConfig *network.EndpointSettings, oldDNSNames []string) {
				if retErr == nil {
					return
				}

				epConfig.DNSNames = oldDNSNames
				if err := ep.UpdateDNSNames(epConfig.DNSNames); err != nil {
					log.G(context.TODO()).WithFields(log.Fields{
						"sandboxID": sid,
						"oldName":   oldName,
						"newName":   newName,
						"error":     err,
					}).Errorf("failed to revert DNSNames update")
				}
			}(ep, epConfig, oldDNSNames)
		}
	}

	daemon.LogContainerEventWithAttributes(container, events.ActionRename, map[string]string{
		"oldName": oldName,
	})
	return nil
}
