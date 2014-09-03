// +build !exclude_graphdriver_zfs

package daemon

import (
	_ "github.com/docker/docker/daemon/graphdriver/zfs"
)
