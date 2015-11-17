package loggerutils

import (
	"fmt"
	"strconv"

	"github.com/docker/docker/daemon/logger"
)

const (
	defaultFailOnStartupError = true // So that we do not break existing behaviour
)

// ParseFailOnStartupErrorFlag parses a log driver flag that determines if
// the driver should ignore possible connection errors during startup
func ParseFailOnStartupErrorFlag(ctx logger.Context) (bool, error) {
	failOnStartupError := ctx.Config["fail-on-startup-error"]
	if failOnStartupError == "" {
		return defaultFailOnStartupError, nil
	}
	failOnStartupErrorFlag, err := strconv.ParseBool(failOnStartupError)
	if err != nil {
		return defaultFailOnStartupError, fmt.Errorf("invalid connect error flag %s: %s", failOnStartupError, err)
	}
	return failOnStartupErrorFlag, nil
}
