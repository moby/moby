package jsonfilelog

import "github.com/moby/moby/v2/daemon/logger"

func init() {
	if err := logger.RegisterLogDriver(Name, New); err != nil {
		panic(err)
	}
	if err := logger.RegisterLogOptValidator(Name, ValidateLogOpt); err != nil {
		panic(err)
	}
}
