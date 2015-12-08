package daemon

import (
	"strings"

	"github.com/docker/distribution/reference"
	derr "github.com/docker/docker/errors"
	tagpkg "github.com/docker/docker/tag"
)

func (d *Daemon) imageNotExistToErrcode(err error) error {
	if dne, isDNE := err.(ErrImageDoesNotExist); isDNE {
		if strings.Contains(dne.RefOrID, "@") {
			return derr.ErrorCodeNoSuchImageHash.WithArgs(dne.RefOrID)
		}
		tag := tagpkg.DefaultTag
		ref, err := reference.ParseNamed(dne.RefOrID)
		if err != nil {
			return derr.ErrorCodeNoSuchImageTag.WithArgs(dne.RefOrID, tag)
		}
		if tagged, isTagged := ref.(reference.Tagged); isTagged {
			tag = tagged.Tag()
		}
		return derr.ErrorCodeNoSuchImageTag.WithArgs(ref.Name(), tag)
	}
	return err
}
