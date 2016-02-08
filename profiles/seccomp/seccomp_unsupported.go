// +build linux,!seccomp

package seccomp

import "github.com/opencontainers/runc/libcontainer/configs"

var (
	// defaultProfile is a nil pointer on unsupported systems.
	defaultProfile *configs.Seccomp
)
