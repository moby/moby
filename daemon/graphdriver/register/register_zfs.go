//go:build (!exclude_graphdriver_zfs && linux) || (!exclude_graphdriver_zfs && freebsd)
// +build !exclude_graphdriver_zfs,linux !exclude_graphdriver_zfs,freebsd

package register // import "github.com/moby/moby/daemon/graphdriver/register"

import (
	// register the zfs driver
	_ "github.com/moby/moby/daemon/graphdriver/zfs"
)
