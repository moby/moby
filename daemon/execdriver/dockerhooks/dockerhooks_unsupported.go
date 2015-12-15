// +build !linux

package dockerhooks

import (
	"github.com/opencontainers/runc/libcontainer/configs"
)

// Prestart function will be called after container process is created but
// before it is started
func Prestart(state configs.HookState, configPath string) error {
	return nil
}

// Poststop function will be called after container process has stopped but
// before it is removed
func Poststop(state configs.HookState, configPath string) error {
	return nil
}

// TotalHooks returns the number of hooks to be used
func TotalHooks() int {
	return 0
}
