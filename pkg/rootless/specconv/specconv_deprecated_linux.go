package specconv // Deprecated: will be removed in the next release.

import (
	"github.com/docker/docker/internal/rootless/specconv"
	"github.com/opencontainers/runtime-spec/specs-go"
)

// Deprecated: will be removed in the next release.
func ToRootfulInRootless(spec *specs.Spec) {
	specconv.ToRootfulInRootless(spec)
}

// Deprecated: will be removed in the next release.
func ToRootless(spec *specs.Spec, v2Controllers []string) error {
	return specconv.ToRootless(spec, v2Controllers)
}
