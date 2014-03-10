package docker

import (
	"github.com/dotcloud/docker/archive"
	"github.com/dotcloud/docker/utils"
)

type Change struct {
	archive.Change
}

// Links come in the format of
// name:alias
func parseLink(rawLink string) (map[string]string, error) {
	return utils.PartParser("name:alias", rawLink)
}
