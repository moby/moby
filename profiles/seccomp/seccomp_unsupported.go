// +build linux,!seccomp

package seccomp

import (
	"github.com/docker/docker/api/types"
	"github.com/opencontainers/runtime-spec/specs-go"
)

// DefaultProfile returns a nil pointer on unsupported systems.
func DefaultProfile(rs *specs.Spec) *types.Seccomp {
	return nil
}
