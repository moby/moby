// +build !exclude_graphdriver_aufs,linux

package register // import "github.com/moby/moby/daemon/graphdriver/register"

import (
	// register the aufs graphdriver
	_ "github.com/moby/moby/daemon/graphdriver/aufs"
)
