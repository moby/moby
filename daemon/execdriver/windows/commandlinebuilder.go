//+build windows

package windows

import (
	"errors"
	"syscall"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/daemon/execdriver"
)

// createCommandLine creates a command line from the Entrypoint and args
// of the ProcessConfig. It escapes the arguments if they are not already
// escaped
func createCommandLine(processConfig *execdriver.ProcessConfig, alreadyEscaped bool) (commandLine string, err error) {
	// While this should get caught earlier, just in case, validate that we
	// have something to run.
	if processConfig.Entrypoint == "" {
		return "", errors.New("No entrypoint specified")
	}

	// Build the command line of the process
	commandLine = processConfig.Entrypoint
	logrus.Debugf("Entrypoint: %s", processConfig.Entrypoint)
	for _, arg := range processConfig.Arguments {
		logrus.Debugf("appending %s", arg)
		if !alreadyEscaped {
			arg = syscall.EscapeArg(arg)
		}
		commandLine += " " + arg
	}

	logrus.Debugf("commandLine: %s", commandLine)
	return commandLine, nil
}
