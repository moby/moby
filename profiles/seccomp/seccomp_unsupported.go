// +build linux,!seccomp

package seccomp

import "github.com/opencontainers/runc/libcontainer/configs"

var (
	defaultSeccompProfile *configs.Seccomp
)
