// +build !exclude_graphdriver_devicemapper,!static_build

package daemon

import (
	_ "github.com/docker/docker/daemon/graphdriver/devmapper"
)
