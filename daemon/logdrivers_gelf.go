//go:build !exclude_logdriver_gelf && (linux || windows)

package daemon

import (
	// register the gelf logging driver
	_ "github.com/moby/moby/v2/daemon/logger/gelf"
)
