// +build windows

package windows

import (
	"errors"

	"github.com/docker/docker/daemon/execdriver"
)

func checkSupportedOptions(c *execdriver.Command) error {
	// Windows doesn't support username
	if c.ProcessConfig.User != "" {
		return errors.New("Windows does not support the username option")
	}

	// TODO Windows: Validate other fields which Windows doesn't support, factor
	// out where applicable per platform.

	return nil
}
