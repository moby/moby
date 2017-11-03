// +build !exclude_graphdriver_devicemapper,!static_build,linux

package register

import (
	// register the devmapper graphdriver
	_ "github.com/moby/moby/daemon/graphdriver/devmapper"
)
