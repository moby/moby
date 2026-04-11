package awslogs

import "github.com/moby/moby/v2/daemon/logger"

func init() {
	if err := logger.RegisterLogDriver(name, New); err != nil {
		panic(err)
	}
	if err := logger.RegisterLogOptValidator(name, ValidateLogOpt); err != nil {
		panic(err)
	}
}
