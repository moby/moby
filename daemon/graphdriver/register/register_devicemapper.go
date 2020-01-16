// +build !exclude_graphdriver_devicemapper,!static_build,linux

package register // import "github.com/moby/moby/daemon/graphdriver/register"

import (
	// register the devmapper graphdriver
	_ "github.com/moby/moby/daemon/graphdriver/devmapper"
)
