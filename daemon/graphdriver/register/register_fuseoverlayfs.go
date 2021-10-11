//go:build !exclude_graphdriver_fuseoverlayfs && linux
// +build !exclude_graphdriver_fuseoverlayfs,linux

package register // import "github.com/docker/docker/daemon/graphdriver/register"

import (
	// register the fuse-overlayfs graphdriver
	_ "github.com/docker/docker/daemon/graphdriver/fuse-overlayfs"
)
