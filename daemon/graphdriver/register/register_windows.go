package register // import "github.com/docker/docker/daemon/graphdriver/register"

import (
	// register the windows graph drivers
	_ "github.com/docker/docker/daemon/graphdriver/lcow"
	_ "github.com/docker/docker/daemon/graphdriver/windows"
)
