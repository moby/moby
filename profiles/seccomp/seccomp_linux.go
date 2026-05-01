package seccomp // import "github.com/docker/docker/profiles/seccomp"

import (
	"github.com/moby/profiles/seccomp"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

// GetDefaultProfile returns the default seccomp profile.
//
//go:fix inline
func GetDefaultProfile(rs *specs.Spec) (*specs.LinuxSeccomp, error) {
	return seccomp.GetDefaultProfile(rs)
}

// LoadProfile takes a json string and decodes the seccomp profile.
//
//go:fix inline
func LoadProfile(body string, rs *specs.Spec) (*specs.LinuxSeccomp, error) {
	return seccomp.LoadProfile(body, rs)
}
