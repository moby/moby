//go:build !exclude_logdriver_etwlogs && windows

package daemon

import (
	// register the etwlogs logging driver
	_ "github.com/moby/moby/v2/daemon/logger/etwlogs"
)
