package seccomp // import "github.com/docker/docker/profiles/seccomp"

import (
	"github.com/moby/profiles/seccomp"
)

// DefaultProfile defines the allowed syscalls for the default seccomp profile.
//
//go:fix inline
func DefaultProfile() *Seccomp {
	return seccomp.DefaultProfile()
}
