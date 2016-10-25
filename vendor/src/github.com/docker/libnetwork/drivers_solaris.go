package libnetwork

import (
	"github.com/docker/libnetwork/drivers/null"
	"github.com/docker/libnetwork/drivers/solaris/bridge"
)

func getInitializers() []initializer {
	return []initializer{
		{bridge.Init, "bridge"},
		{null.Init, "null"},
	}
}
