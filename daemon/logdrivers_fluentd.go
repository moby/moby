//go:build !exclude_logdriver_fluentd && (linux || windows)

package daemon

import (
	// register the fluentd logging driver
	_ "github.com/moby/moby/v2/daemon/logger/fluentd"
)
