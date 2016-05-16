// +build !exclude_storage_btrfs,linux

package register

import (
	// register the btrfs storage
	_ "github.com/docker/docker/daemon/storage/btrfs"
)
