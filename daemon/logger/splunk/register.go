package splunk

import "github.com/moby/moby/v2/daemon/logger"

func init() {
	if err := logger.RegisterLogDriver(driverName, New); err != nil {
		panic(err)
	}
	if err := logger.RegisterLogOptValidator(driverName, ValidateLogOpt); err != nil {
		panic(err)
	}
}
