// +build !exclude_graphdriver_overlay,linux

package register

import (
	// register the overlay graphdriver
	_ "github.com/docker/docker/monolith/daemon/graphdriver/overlay"
	_ "github.com/docker/docker/monolith/daemon/graphdriver/overlay2"
)
