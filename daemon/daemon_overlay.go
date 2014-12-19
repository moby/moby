// +build !exclude_graphdriver_overlay

package daemon

import (
	_ "github.com/docker/docker/storage/overlay"
)
