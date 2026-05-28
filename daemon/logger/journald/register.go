//go:build linux

package journald

import "github.com/moby/moby/v2/daemon/logger"

func init() {
	if err := logger.RegisterLogDriver(name, New); err != nil {
		panic(err)
	}
	if err := logger.RegisterLogOptValidator(name, validateLogOpt); err != nil {
		panic(err)
	}
}
