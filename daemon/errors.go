package daemon

import (
	"strings"

	derr "github.com/docker/docker/errors"
	"github.com/docker/docker/reference"
)

func (d *Daemon) imageNotExistToErrcode(err error) error {
	if dne, isDNE := err.(ErrImageDoesNotExist); isDNE {
		if strings.Contains(dne.RefOrID, "@") {
			return derr.ErrorCodeNoSuchImageHash.WithArgs(dne.RefOrID)
		}
		tag := reference.DefaultTag
		ref, err := reference.ParseNamed(dne.RefOrID)
		if err != nil {
			return derr.ErrorCodeNoSuchImageTag.WithArgs(dne.RefOrID, tag)
		}
		if tagged, isTagged := ref.(reference.NamedTagged); isTagged {
			tag = tagged.Tag()
		}
		return derr.ErrorCodeNoSuchImageTag.WithArgs(ref.Name(), tag)
	}
	return err
}
