// +build !exclude_storage_overlay,linux

package register

import (
	// register the overlay storage
	_ "github.com/docker/docker/daemon/storage/overlay"
)
