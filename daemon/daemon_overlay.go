// +build !exclude_graphdriver_overlay

package daemon

import (
	_ "github.com/docker/docker/daemon/graphdriver/overlay"
)
