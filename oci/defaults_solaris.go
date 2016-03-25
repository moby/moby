package oci

import (
	"github.com/opencontainers/specs/specs-go"
)

// DefaultSpec returns default oci spec used by docker.
func DefaultSpec() specs.Spec {
	s := specs.Spec{}
	return s
}
