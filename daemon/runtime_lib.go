// +build !autogen

package daemon

const (
	// DefaultRuntimeBinary is the default runtime to be used by
	// containerd if none is specified
	DefaultRuntimeBinary = "runc"

	// DefaultShimBinary is the default shim to be used by containerd if none
	// is specified
	DefaultShimBinary = "containerd-shim"
)
