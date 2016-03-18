package daemon

import (
	"github.com/docker/docker/container"
	"github.com/docker/docker/libcontainerd"
)

// platformConstructExitStatus returns a platform specific exit status structure
func platformConstructExitStatus(e libcontainerd.StateInfo) *container.ExitStatus {
	return &container.ExitStatus{
		ExitCode:  int(e.ExitCode),
		OOMKilled: e.OOMKilled,
	}
}
