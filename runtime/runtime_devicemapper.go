// +build !exclude_graphdriver_devicemapper

package runtime

import (
	_ "github.com/dotcloud/docker/runtime/graphdriver/devmapper"
)
