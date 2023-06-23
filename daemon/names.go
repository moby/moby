package daemon // import "github.com/docker/docker/daemon"

import (
	"context"
	"fmt"
	"strings"

	"github.com/containerd/containerd/log"
	"github.com/docker/docker/container"
	"github.com/docker/docker/daemon/names"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/pkg/namesgenerator"
	"github.com/docker/docker/pkg/stringid"
	"github.com/pkg/errors"
)

var (
	validContainerNameChars   = names.RestrictedNameChars
	validContainerNamePattern = names.RestrictedNamePattern
)

func (daemon *Daemon) registerName(container *container.Container) error {
	if daemon.Exists(container.ID) {
		return fmt.Errorf("Container is already loaded")
	}
	if err := validateID(container.ID); err != nil {
		return err
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
		if errors.Is(err, container.ErrNameReserved) {
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
	for i := 0; i < 6; i++ {
		name = namesgenerator.GetRandomName(i)
		if name[0] != '/' {
			name = "/" + name
		}

		if err := daemon.containersReplica.ReserveName(name, id); err != nil {
			if errors.Is(err, container.ErrNameReserved) {
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

func validateID(id string) error {
	if id == "" {
		return fmt.Errorf("Invalid empty id")
	}
	return nil
}
