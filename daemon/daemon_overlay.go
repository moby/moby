// +build !exclude_graphdriver_overlay,linux

package daemon

import (
	// register the overlay graphdriver
	_ "github.com/docker/docker/daemon/graphdriver/overlay"
)
