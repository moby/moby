// +build !exclude_graphdriver_btrfs

package daemon

import (
	_ "github.com/dotcloud/docker/daemon/graphdriver/btrfs"
)
