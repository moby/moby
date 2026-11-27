//go:build !exclude_logdriver_syslog && (linux || windows)

package daemon

import (
	// register the syslog logging driver
	_ "github.com/moby/moby/v2/daemon/logger/syslog"
)
