//go:build windows

package etwlogs

import "github.com/moby/moby/v2/daemon/logger"

func init() {
	if err := logger.RegisterLogDriver(name, New); err != nil {
		panic(err)
	}
}
