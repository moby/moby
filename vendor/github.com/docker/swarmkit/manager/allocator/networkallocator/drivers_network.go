// +build linux windows

package networkallocator

import (
	"github.com/docker/libnetwork/drivers/overlay/ovmanager"
	"github.com/docker/libnetwork/drivers/remote"
)

func getInitializers() []initializer {
	return []initializer{
		{remote.Init, "remote"},
		{ovmanager.Init, "overlay"},
	}
}
