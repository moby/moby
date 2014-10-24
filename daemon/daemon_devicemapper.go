// +build !exclude_graphdriver_devicemapper

package daemon

import (
	_ "github.com/docker/docker/daemon/graphdriver/devmapper"
)
