// +build linux freebsd

package daemon

import (
	"github.com/docker/docker/container"
	"github.com/docker/docker/daemon/execdriver"
	"github.com/docker/engine-api/types"
)

// setPlatformSpecificExecProcessConfig sets platform-specific fields in the
// ProcessConfig structure.
func setPlatformSpecificExecProcessConfig(config *types.ExecConfig, container *container.Container, pc *execdriver.ProcessConfig) {
	user := config.User
	if len(user) == 0 {
		user = container.Config.User
	}

	pc.User = user
	pc.Privileged = config.Privileged
}
