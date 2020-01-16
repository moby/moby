package register // import "github.com/moby/moby/daemon/graphdriver/register"

import (
	// register the windows graph drivers
	_ "github.com/moby/moby/daemon/graphdriver/lcow"
	_ "github.com/moby/moby/daemon/graphdriver/windows"
)
