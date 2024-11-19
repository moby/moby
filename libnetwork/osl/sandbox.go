// Package osl describes structures and interfaces which abstract os entities
package osl

// SandboxType specifies the type(s) of the sandbox as a bitmask.
// This can be used to apply special configs.
type SandboxType int

const (
	// SandboxTypeIngress indicates that the sandbox is for the ingress
	SandboxTypeIngress SandboxType = 1 << iota
	// SandboxTypeLoadBalancer indicates that the sandbox is a load balancer
	SandboxTypeLoadBalancer
)

type Iface struct {
	SrcName, DstPrefix string
}

// IfaceOption is a function option type to set interface options.
type IfaceOption func(i *Interface) error

// NeighOption is a function option type to set neighbor options.
type NeighOption func(nh *neigh)
