package runtime

import (
	"strconv"

	"github.com/containerd/containerd/typeurl"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

func init() {
	// register TypeUrls for commonly marshaled external types
	major := strconv.Itoa(specs.VersionMajor)
	typeurl.Register(&specs.Spec{}, "opencontainers/runtime-spec", major, "Spec")
	typeurl.Register(&specs.Process{}, "opencontainers/runtime-spec", major, "Process")
	typeurl.Register(&specs.LinuxResources{}, "opencontainers/runtime-spec", major, "LinuxResources")
	typeurl.Register(&specs.WindowsResources{}, "opencontainers/runtime-spec", major, "WindowsResources")
}
