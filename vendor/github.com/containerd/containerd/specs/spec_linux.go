package specs

import oci "github.com/opencontainers/runtime-spec/specs-go"

type (
	// ProcessSpec aliases the platform process specs
	ProcessSpec oci.Process
	// Spec aliases the platform oci spec
	Spec oci.Spec
	// Rlimit aliases the platform resource limit
	Rlimit oci.LinuxRlimit
)
