package daemon

import (
	"strings"

	derr "github.com/docker/docker/errors"
	"github.com/docker/docker/graph/tags"
	"github.com/docker/docker/pkg/parsers"
)

func (d *Daemon) graphNotExistToErrcode(imageName string, err error) error {
	if d.Graph().IsNotExist(err, imageName) {
		if strings.Contains(imageName, "@") {
			return derr.ErrorCodeNoSuchImageHash.WithArgs(imageName)
		}
		img, tag := parsers.ParseRepositoryTag(imageName)
		if tag == "" {
			tag = tags.DefaultTag
		}
		return derr.ErrorCodeNoSuchImageTag.WithArgs(img, tag)
	}
	return err
}
