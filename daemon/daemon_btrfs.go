// +build !exclude_graphdriver_btrfs,linux

package daemon

import (
	// register the btrfs graphdriver
	_ "github.com/docker/docker/daemon/graphdriver/btrfs"
)
