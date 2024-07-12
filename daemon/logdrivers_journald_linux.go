//go:build linux && !no_systemd

package daemon // import "github.com/docker/docker/daemon"

import (
	// Importing packages here only to make sure their init gets called and
	// therefore they register themselves to the logdriver factory.
	_ "github.com/docker/docker/daemon/logger/journald"
)
