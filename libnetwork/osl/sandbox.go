// Package osl describes structures and interfaces which abstract os entities
package osl

// SandboxType specify the time of the sandbox, this can be used to apply special configs
type SandboxType int

const (
	// SandboxTypeIngress indicates that the sandbox is for the ingress
	SandboxTypeIngress = iota
	// SandboxTypeLoadBalancer indicates that the sandbox is a load balancer
	SandboxTypeLoadBalancer = iota
)

type Iface struct {
	SrcName, DstPrefix string
}

// IfaceOption is a function option type to set interface options.
type IfaceOption func(i *Interface) error

// NeighOption is a function option type to set neighbor options.
type NeighOption func(nh *neigh)
