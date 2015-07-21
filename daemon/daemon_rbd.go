// +build !exclude_graphdriver_rbd

package daemon

import (
	_ "github.com/docker/docker/daemon/graphdriver/rbd"
)
