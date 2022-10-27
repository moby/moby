package libnetwork

import (
	"github.com/docker/docker/libnetwork/drivers/null"
	"github.com/docker/docker/libnetwork/drivers/remote"
)

func getInitializers() []initializer {
	return []initializer{
		{null.Init, "null"},
		{remote.Init, "remote"},
	}
}
