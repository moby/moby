package daemon

import (
	"github.com/docker/docker/daemon/execdriver"
	"github.com/docker/docker/runconfig"
)

// setPlatformSpecificExecProcessConfig sets platform-specific fields in the
// ProcessConfig structure. This is a no-op on Windows
func setPlatformSpecificExecProcessConfig(config *runconfig.ExecConfig, container *Container, pc *execdriver.ProcessConfig) {
}
