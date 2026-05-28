//go:build !exclude_graphdriver_btrfs && linux

package register

import (
	// register the btrfs graphdriver
	_ "github.com/moby/moby/v2/daemon/graphdriver/btrfs"
)
