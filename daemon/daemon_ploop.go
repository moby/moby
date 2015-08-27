// +build !exclude_graphdriver_ploop,linux

package daemon

import (
	// register the ploop graphdriver
	_ "github.com/docker/docker/daemon/graphdriver/ploop"
)
