// +build !exclude_storage_devicemapper,linux

package register

import (
	// register the devmapper storage
	_ "github.com/docker/docker/daemon/storage/devmapper"
)
