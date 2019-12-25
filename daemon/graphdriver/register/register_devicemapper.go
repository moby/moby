// +build !exclude_graphdriver_devicemapper,!static_build,linux

package register // import "github.com/docker/docker/daemon/graphdriver/register"

import (
	// register the devmapper graphdriver
	_ "github.com/docker/docker/daemon/graphdriver/devmapper"
)
