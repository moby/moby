// +build !exclude_graphdriver_ceph

package daemon

import (
	_ "github.com/docker/docker/daemon/graphdriver/ceph"
)
