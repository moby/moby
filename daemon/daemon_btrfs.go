// +build !exclude_graphdriver_btrfs

package daemon

import (
	_ "github.com/docker/docker/storage/btrfs"
)
