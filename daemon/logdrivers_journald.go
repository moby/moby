//go:build !exclude_logdriver_journald && linux

package daemon

import (
	// register the journald logging driver
	_ "github.com/moby/moby/v2/daemon/logger/journald"
)
