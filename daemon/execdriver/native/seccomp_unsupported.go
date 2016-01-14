// +build linux,!seccomp

package native

import "github.com/opencontainers/runc/libcontainer/configs"

var (
	defaultSeccompProfile *configs.Seccomp
)
