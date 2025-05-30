//go:build !exclude_graphdriver_fuseoverlayfs && linux

package register

import (
	// register the fuse-overlayfs graphdriver
	_ "github.com/docker/docker/daemon/graphdriver/fuse-overlayfs"
)
