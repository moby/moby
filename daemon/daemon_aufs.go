// +build !exclude_graphdriver_aufs,linux

package daemon

import (
	// register the aufs graphdriver
	_ "github.com/docker/docker/daemon/graphdriver/aufs"
)
