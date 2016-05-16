// +build !exclude_storage_zfs,linux !exclude_storage_zfs,freebsd

package register

import (
	// register the zfs driver
	_ "github.com/docker/docker/daemon/storage/zfs"
)
