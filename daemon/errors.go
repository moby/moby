package daemon

import (
	"fmt"
	"strings"

	"github.com/docker/docker/api/errors"
	"github.com/docker/docker/reference"
)

func (d *Daemon) imageNotExistToErrcode(err error) error {
	if dne, isDNE := err.(ErrImageDoesNotExist); isDNE {
		if strings.Contains(dne.RefOrID, "@") {
			e := fmt.Errorf("No such image: %s", dne.RefOrID)
			return errors.NewRequestNotFoundError(e)
		}
		tag := reference.DefaultTag
		ref, err := reference.ParseNamed(dne.RefOrID)
		if err != nil {
			e := fmt.Errorf("No such image: %s:%s", dne.RefOrID, tag)
			return errors.NewRequestNotFoundError(e)
		}
		if tagged, isTagged := ref.(reference.NamedTagged); isTagged {
			tag = tagged.Tag()
		}
		e := fmt.Errorf("No such image: %s:%s", ref.Name(), tag)
		return errors.NewRequestNotFoundError(e)
	}
	return err
}

type errNotRunning struct {
	containerID string
}

func (e errNotRunning) Error() string {
	return fmt.Sprintf("Container %s is not running", e.containerID)
}

func (e errNotRunning) ContainerIsRunning() bool {
	return false
}

func errContainerIsRestarting(containerID string) error {
	err := fmt.Errorf("Container %s is restarting, wait until the container is running", containerID)
	return errors.NewRequestConflictError(err)
}

func errExecNotFound(id string) error {
	err := fmt.Errorf("No such exec instance '%s' found in daemon", id)
	return errors.NewRequestNotFoundError(err)
}

func errExecPaused(id string) error {
	err := fmt.Errorf("Container %s is paused, unpause the container before exec", id)
	return errors.NewRequestConflictError(err)
}
