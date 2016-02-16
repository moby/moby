package daemon

import (
	"fmt"
	"strings"

	"github.com/docker/docker/errors"
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
