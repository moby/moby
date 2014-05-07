// +build !exclude_graphdriver_devicemapper

package daemon

import (
	_ "github.com/dotcloud/docker/daemon/graphdriver/devmapper"
)
