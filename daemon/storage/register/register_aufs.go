// +build !exclude_storage_aufs,linux

package register

import (
	// register the aufs storage
	_ "github.com/docker/docker/daemon/storage/aufs"
)
