package register

import (
	// register the windows graph driver
	_ "github.com/docker/docker/daemon/graphdriver/lcow"
	_ "github.com/docker/docker/daemon/graphdriver/windows"
)
