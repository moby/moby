//go:build !linux
// +build !linux

package specconv

import (
	"runtime"

	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
)

// ToRootless converts spec to be compatible with "rootless" runc.
// * Remove /sys mount
// * Remove cgroups
//
// See docs/rootless.md for the supported runc revision.
func ToRootless(spec *specs.Spec) error {
	return errors.Errorf("not implemented on on %s", runtime.GOOS)
}
