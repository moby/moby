//go:build !exclude_graphdriver_overlay && linux
// +build !exclude_graphdriver_overlay,linux

package register // import "github.com/docker/docker/daemon/graphdriver/register"

import (
	// register the overlay graphdriver
	_ "github.com/docker/docker/daemon/graphdriver/overlay"
)
