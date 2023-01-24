package libnetwork

import (
	"github.com/docker/docker/libnetwork/drivers/null"
)

func getInitializers() []initializer {
	return []initializer{
		{null.Register, "null"},
	}
}
