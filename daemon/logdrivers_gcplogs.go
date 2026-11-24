//go:build !exclude_logdriver_gcplogs && (linux || windows)

package daemon

import (
	// register the gcplogs logging driver
	_ "github.com/moby/moby/v2/daemon/logger/gcplogs"
)
