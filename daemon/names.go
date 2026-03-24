package daemon

import (
	"context"
	"strings"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/containerd/log"
	"github.com/moby/moby/v2/daemon/container"
	"github.com/moby/moby/v2/daemon/internal/stringid"
	"github.com/moby/moby/v2/daemon/names"
	"github.com/moby/moby/v2/errdefs"
	"github.com/moby/moby/v2/internal/namesgenerator"
	"github.com/pkg/errors"
)

var (
	validContainerNameChars   = names.RestrictedNameChars
	validContainerNamePattern = names.RestrictedNamePattern
)

func (daemon *Daemon) registerName(container *container.Container) error {
	if container.ID == "" {
		return errors.New("invalid empty id")
	}
	if daemon.containers.Get(container.ID) != nil {
		// TODO(thaJeztah): should this be a panic (duplicate IDs due to invalid state on disk?)
		// TODO(thaJeztah): should this also check for container.ID being a prefix of another container's ID? (daemon.containersReplica.GetByPrefix); only should happen due to corruption / truncated ID.
		return errors.New("container is already loaded")
	}
	if container.Name == "" {
		name, err := daemon.generateAndReserveName(container.ID)
		if err != nil {
			return err
		}
		container.Name = name
		return nil
	}
	return daemon.containersReplica.ReserveName(container.Name, container.ID)
}

func (daemon *Daemon) generateIDAndName(name string) (string, string, error) {
	var (
		err error
		id  = stringid.GenerateRandomID()
	)

	if name == "" {
		if name, err = daemon.generateAndReserveName(id); err != nil {
			return "", "", err
		}
		return id, name, nil
	}

	if name, err = daemon.reserveName(id, name); err != nil {
		return "", "", err
	}

	return id, name, nil
}

func (daemon *Daemon) reserveName(id, name string) (string, error) {
	if !validContainerNamePattern.MatchString(strings.TrimPrefix(name, "/")) {
		return "", errdefs.InvalidParameter(errors.Errorf("Invalid container name (%s), only %s are allowed", name, validContainerNameChars))
	}
	if name[0] != '/' {
		name = "/" + name
	}

	if err := daemon.containersReplica.ReserveName(name, id); err != nil {
		if cerrdefs.IsConflict(err) {
			id, err := daemon.containersReplica.Snapshot().GetID(name)
			if err != nil {
				log.G(context.TODO()).Errorf("got unexpected error while looking up reserved name: %v", err)
				return "", err
			}
			return "", nameConflictError{id: id, name: name}
		}
		return "", errors.Wrapf(err, "error reserving name: %q", name)
	}
	return name, nil
}

func (daemon *Daemon) releaseName(name string) {
	daemon.containersReplica.ReleaseName(name)
}

func (daemon *Daemon) generateAndReserveName(id string) (string, error) {
	var name string
	for i := range 6 {
		name = namesgenerator.GetRandomName(i)
		if name[0] != '/' {
			name = "/" + name
		}

		if err := daemon.containersReplica.ReserveName(name, id); err != nil {
			if cerrdefs.IsConflict(err) {
				continue
			}
			return "", err
		}
		return name, nil
	}

	name = "/" + stringid.TruncateID(id)
	if err := daemon.containersReplica.ReserveName(name, id); err != nil {
		return "", err
	}
	return name, nil
}
