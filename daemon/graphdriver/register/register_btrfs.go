// +build !exclude_graphdriver_btrfs,linux

package register // import "github.com/docker/docker/daemon/graphdriver/register"

import (
	// register the btrfs graphdriver
	_ "github.com/docker/docker/daemon/graphdriver/btrfs"
)
