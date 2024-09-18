package daemon // import "github.com/docker/docker/daemon"

import (
	"context"
	"fmt"
	"strings"

	"github.com/containerd/log"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/container"
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

	ctr, err := daemon.GetContainer(oldName)
	if err != nil {
		return err
	}
	ctr.Lock()
	defer ctr.Unlock()

	// Canonicalize name for comparing.
	if newName[0] != '/' {
		newName = "/" + newName
	}
	if ctr.Name == newName {
		return errdefs.InvalidParameter(errors.New("Renaming a container with the same name as its current name"))
	}

	links := map[string]*container.Container{}
	for k, v := range daemon.linkIndex.children(ctr) {
		if !strings.HasPrefix(k, ctr.Name) {
			return errdefs.InvalidParameter(errors.Errorf("Linked container %s does not match parent %s", k, ctr.Name))
		}
		links[strings.TrimPrefix(k, ctr.Name)] = v
	}

	newName, err = daemon.reserveName(ctr.ID, newName)
	if err != nil {
		return errors.Wrap(err, "Error when allocating new name")
	}

	for k, v := range links {
		daemon.containersReplica.ReserveName(newName+k, v.ID)
		daemon.linkIndex.link(ctr, v, newName+k)
	}

	oldName = ctr.Name
	ctr.Name = newName

	defer func() {
		if retErr != nil {
			ctr.Name = oldName
			daemon.reserveName(ctr.ID, oldName)
			for k, v := range links {
				daemon.containersReplica.ReserveName(oldName+k, v.ID)
				daemon.linkIndex.link(ctr, v, oldName+k)
				daemon.linkIndex.unlink(newName+k, v, ctr)
				daemon.containersReplica.ReleaseName(newName + k)
			}
			daemon.releaseName(newName)
		} else {
			daemon.releaseName(oldName)
		}
	}()

	for k, v := range links {
		daemon.linkIndex.unlink(oldName+k, v, ctr)
		daemon.containersReplica.ReleaseName(oldName + k)
	}
	if err := ctr.CheckpointTo(context.TODO(), daemon.containersReplica); err != nil {
		return err
	}

	if !ctr.Running {
		daemon.LogContainerEventWithAttributes(ctr, events.ActionRename, map[string]string{
			"oldName": oldName,
		})
		return nil
	}

	defer func() {
		if retErr != nil {
			ctr.Name = oldName
			if err := ctr.CheckpointTo(context.WithoutCancel(context.TODO()), daemon.containersReplica); err != nil {
				log.G(context.TODO()).WithFields(log.Fields{
					"containerID": ctr.ID,
					"error":       err,
				}).Error("failed to write container state to disk during rename")
			}
		}
	}()

	if sid := ctr.NetworkSettings.SandboxID; sid != "" && daemon.netController != nil {
		sb, err := daemon.netController.SandboxByID(sid)
		if err != nil {
			return err
		}
		if err = sb.Rename(strings.TrimPrefix(ctr.Name, "/")); err != nil {
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

		for nwName, epConfig := range ctr.NetworkSettings.Networks {
			nw, err := daemon.FindNetwork(nwName)
			if err != nil {
				return err
			}

			ep := sb.GetEndpoint(epConfig.EndpointID)
			if ep == nil {
				return fmt.Errorf("no endpoint attached to network %s found", nw.Name())
			}

			oldDNSNames := make([]string, len(epConfig.DNSNames))
			copy(oldDNSNames, epConfig.DNSNames)

			epConfig.DNSNames = buildEndpointDNSNames(ctr, epConfig.Aliases)
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

	daemon.LogContainerEventWithAttributes(ctr, events.ActionRename, map[string]string{
		"oldName": oldName,
	})
	return nil
}
