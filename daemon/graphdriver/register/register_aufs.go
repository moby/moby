// +build !exclude_graphdriver_aufs,linux

package register // import "github.com/docker/docker/daemon/graphdriver/register"

import (
	// register the aufs graphdriver
	_ "github.com/docker/docker/daemon/graphdriver/aufs"
)
