//go:build (!exclude_graphdriver_zfs && linux) || (!exclude_graphdriver_zfs && freebsd)

package register

import (
	// register the zfs driver
	_ "github.com/moby/moby/v2/daemon/graphdriver/zfs"
)
