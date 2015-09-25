package libnetwork

import "github.com/docker/libnetwork/drivers/windows"

func getInitializers() []initializer {
	return []initializer{
		{windows.Init, "windows"},
	}
}
