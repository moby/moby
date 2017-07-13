//+build ignore

package v17_06_1

import (
	"github.com/containerd/containerd/runtime"
	"github.com/opencontainers/runc/libcontainer"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

//go:generate -command rewrite go run ../gen/rewrite-structs.go --

//go:generate rewrite spec_gen.go .Process.Capabilities->linuxCapabilities .Linux.Resources.Memory.Swappiness->memorySwappiness .Linux.Seccomp.Syscalls->linuxSyscalls
type Spec specs.Spec

//go:generate rewrite process_state_gen.go .Capabilities->linuxCapabilities
type ProcessState runtime.ProcessState

//go:generate rewrite state_gen.go .Config.Capabilities->linuxCapabilities .Config.Cgroups.MemorySwappiness->memorySwappiness
type State libcontainer.State
