package libnetwork

import (
	"github.com/moby/moby/libnetwork/drivers/null"
	"github.com/moby/moby/libnetwork/drivers/remote"
)

func getInitializers(experimental bool) []initializer {
	return []initializer{
		{null.Init, "null"},
		{remote.Init, "remote"},
	}
}
