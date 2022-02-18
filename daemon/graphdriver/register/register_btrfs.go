//go:build !exclude_graphdriver_btrfs && linux
// +build !exclude_graphdriver_btrfs,linux

package register // import "github.com/moby/moby/daemon/graphdriver/register"

import (
	// register the btrfs graphdriver
	_ "github.com/moby/moby/daemon/graphdriver/btrfs"
)
