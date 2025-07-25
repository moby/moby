package seccomp

import (
	"github.com/moby/profiles/seccomp"
	"github.com/opencontainers/runtime-spec/specs-go"
)

// DefaultProfile defines the allowed syscalls for the default seccomp profile.
//
// Deprecated: use [seccomp.DefaultProfile].
func DefaultProfile() *seccomp.Seccomp {
	return seccomp.DefaultProfile()
}

// GetDefaultProfile returns the default seccomp profile.
//
// Deprecated: use [seccomp.GetDefaultProfile].
func GetDefaultProfile(rs *specs.Spec) (*specs.LinuxSeccomp, error) {
	return seccomp.GetDefaultProfile(rs)
}

// LoadProfile takes a json string and decodes the seccomp profile.
//
// Deprecated: use [seccomp.LoadProfile].
func LoadProfile(body string, rs *specs.Spec) (*specs.LinuxSeccomp, error) {
	return seccomp.LoadProfile(body, rs)
}
