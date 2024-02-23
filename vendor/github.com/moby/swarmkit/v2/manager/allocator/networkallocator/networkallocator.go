package networkallocator

import (
	"github.com/moby/swarmkit/v2/api"
)

const (
	// PredefinedLabel identifies internally allocated swarm networks
	// corresponding to the node-local predefined networks on the host.
	PredefinedLabel = "com.docker.swarm.predefined"
)

// PredefinedNetworkData contains the minimum set of data needed
// to create the correspondent predefined network object in the store.
type PredefinedNetworkData struct {
	Name   string
	Driver string
}

// ServiceAllocationOpts is struct used for functional options in
// IsServiceAllocated
type ServiceAllocationOpts struct {
	OnInit bool
}

// OnInit is called for allocator initialization stage
func OnInit(options *ServiceAllocationOpts) {
	options.OnInit = true
}

// NetworkAllocator provides network model specific allocation functionality.
type NetworkAllocator interface {
	//
	// Network Allocation
	//

	// IsAllocated returns if the passed network has been allocated or not.
	IsAllocated(n *api.Network) bool

	// Allocate allocates all the necessary resources both general
	// and driver-specific which may be specified in the NetworkSpec
	Allocate(n *api.Network) error

	// Deallocate frees all the general and driver specific resources
	// which were assigned to the passed network.
	Deallocate(n *api.Network) error

	//
	// Service Allocation
	//

	// IsServiceAllocated returns false if the passed service
	// needs to have network resources allocated/updated.
	IsServiceAllocated(s *api.Service, flags ...func(*ServiceAllocationOpts)) bool

	// AllocateService allocates all the network resources such as virtual
	// IP and ports needed by the service.
	AllocateService(s *api.Service) (err error)

	// DeallocateService de-allocates all the network resources such as
	// virtual IP and ports associated with the service.
	DeallocateService(s *api.Service) error

	//
	// Task Allocation
	//

	// IsTaskAllocated returns if the passed task has its network
	// resources allocated or not.
	IsTaskAllocated(t *api.Task) bool

	// AllocateTask allocates all the endpoint resources for all the
	// networks that a task is attached to.
	AllocateTask(t *api.Task) error

	// DeallocateTask releases all the endpoint resources for all the
	// networks that a task is attached to.
	DeallocateTask(t *api.Task) error

	// AllocateAttachment Allocates a load balancer endpoint for the node
	AllocateAttachment(node *api.Node, networkAttachment *api.NetworkAttachment) error

	// DeallocateAttachment Deallocates a load balancer endpoint for the node
	DeallocateAttachment(node *api.Node, networkAttachment *api.NetworkAttachment) error

	// IsAttachmentAllocated If lb endpoint is allocated on the node
	IsAttachmentAllocated(node *api.Node, networkAttachment *api.NetworkAttachment) bool
}

// Config is used to store network related cluster config in the Manager.
type Config struct {
	// DefaultAddrPool specifies default subnet pool for global scope networks
	DefaultAddrPool []string

	// SubnetSize specifies the subnet size of the networks created from
	// the default subnet pool
	SubnetSize uint32

	// VXLANUDPPort specifies the UDP port number for VXLAN traffic
	VXLANUDPPort uint32
}

// DriverValidator validates whether a network driver spec is supported by the
// network provider.
type DriverValidator interface {
	ValidateNetworkDriver(*api.Driver) error
	ValidateIngressNetworkDriver(*api.Driver) error
	ValidateIPAMDriver(*api.Driver) error
}

// Provider provides network allocation functionality.
type Provider interface {
	DriverValidator
	PredefinedNetworks() []PredefinedNetworkData
	SetDefaultVXLANUDPPort(uint32) error
	NewAllocator(*Config) (NetworkAllocator, error)
}

// IsIngressNetwork check if the network is an ingress network
func IsIngressNetwork(nw *api.Network) bool {
	if nw.Spec.Ingress {
		return true
	}
	// Check if legacy defined ingress network
	_, ok := nw.Spec.Annotations.Labels["com.docker.swarm.internal"]
	return ok && nw.Spec.Annotations.Name == "ingress"
}

// IsIngressNetworkNeeded checks whether the service requires the routing-mesh
func IsIngressNetworkNeeded(s *api.Service) bool {
	if s == nil {
		return false
	}

	if s.Spec.Endpoint == nil {
		return false
	}

	for _, p := range s.Spec.Endpoint.Ports {
		// The service to which this task belongs is trying to
		// expose ports with PublishMode as Ingress to the
		// external world. Automatically attach the task to
		// the ingress network.
		if p.PublishMode == api.PublishModeIngress {
			return true
		}
	}

	return false
}
