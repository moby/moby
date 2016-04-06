// +build linux,!seccomp

package seccomp

import "github.com/docker/engine-api/types"

var (
	// DefaultProfile is a nil pointer on unsupported systems.
	DefaultProfile *types.Seccomp
)
