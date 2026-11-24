//go:build !exclude_logdriver_splunk && (linux || windows)

package daemon

import (
	// register the splunk logging driver
	_ "github.com/moby/moby/v2/daemon/logger/splunk"
)
