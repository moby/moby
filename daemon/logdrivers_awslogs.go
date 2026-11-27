//go:build !exclude_logdriver_awslogs && (linux || windows)

package daemon

import (
	// register the awslogs logging driver
	_ "github.com/moby/moby/v2/daemon/logger/awslogs"
)
