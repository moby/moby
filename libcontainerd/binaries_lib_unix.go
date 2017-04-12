// +build linux solaris
// +build !autogen

package libcontainerd

var (
	containerdBinary       = "containerd"
	containerdShimBinary   = "containerd-shim"
	containerdCtrBinary    = "containerd-ctr"
	containerdPidFilename  = "containerd.pid"
	containerdSockFilename = "containerd.sock"
)
