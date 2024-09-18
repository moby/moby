package networkallocator

import (
	"errors"

	"github.com/moby/swarmkit/v2/api"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// InertProvider is a network allocator [Provider] which does not allocate networks.
type InertProvider struct{}

var _ Provider = InertProvider{}

// NewAllocator returns an instance of [Inert].
func (InertProvider) NewAllocator(*Config) (NetworkAllocator, error) {
	return Inert{}, nil
}

// PredefinedNetworks returns a nil slice.
func (InertProvider) PredefinedNetworks() []PredefinedNetworkData {
	return nil
}

// SetDefaultVXLANUDPPort is a no-op.
func (InertProvider) SetDefaultVXLANUDPPort(uint32) error {
	return nil
}

// ValidateIPAMDriver returns an InvalidArgument error unless d is nil.
func (InertProvider) ValidateIPAMDriver(d *api.Driver) error {
	if d == nil {
		return nil
	}
	return status.Errorf(codes.InvalidArgument, "IPAM drivers are unavailable")
}

// ValidateIngressNetworkDriver returns an InvalidArgument error unless d is nil.
func (InertProvider) ValidateIngressNetworkDriver(d *api.Driver) error {
	if d == nil {
		return nil
	}
	return status.Errorf(codes.InvalidArgument, "ingress network drivers are unavailable")
}

// ValidateNetworkDriver returns an InvalidArgument error unless d is nil.
func (InertProvider) ValidateNetworkDriver(d *api.Driver) error {
	if d == nil {
		return nil
	}
	return status.Errorf(codes.InvalidArgument, "ingress network drivers are unavailable")
}

// Inert is a [NetworkAllocator] which does not allocate networks.
type Inert struct{}

var _ NetworkAllocator = Inert{}

var errUnavailable = errors.New("network support is unavailable")

// Allocate returns an error unless n.Spec.Ingress is true.
func (Inert) Allocate(n *api.Network) error {
	if n.Spec.Ingress {
		return nil
	}
	return errUnavailable
}

// AllocateAttachment unconditionally returns an error.
func (Inert) AllocateAttachment(node *api.Node, networkAttachment *api.NetworkAttachment) error {
	return errUnavailable
}

// AllocateService succeeds iff the service specifies no network attachments.
func (Inert) AllocateService(s *api.Service) error {
	if len(s.Spec.Task.Networks) > 0 || len(s.Spec.Networks) > 0 {
		return errUnavailable
	}
	return nil
}

// AllocateTask succeeds iff the task specifies no network attachments.
func (Inert) AllocateTask(t *api.Task) error {
	if len(t.Spec.Networks) > 0 {
		return errUnavailable
	}
	return nil
}

// Deallocate does nothing, successfully.
func (Inert) Deallocate(n *api.Network) error {
	return nil
}

// DeallocateAttachment does nothing, successfully.
func (Inert) DeallocateAttachment(node *api.Node, networkAttachment *api.NetworkAttachment) error {
	return nil
}

// DeallocateService does nothing, successfully.
func (Inert) DeallocateService(s *api.Service) error {
	return nil
}

// DeallocateTask does nothing, successfully.
func (Inert) DeallocateTask(t *api.Task) error {
	return nil
}

// IsAllocated returns true iff [Inert.Allocate] would return nil.
func (Inert) IsAllocated(n *api.Network) bool {
	return (Inert{}).Allocate(n) == nil
}

// IsAttachmentAllocated returns false.
func (Inert) IsAttachmentAllocated(node *api.Node, networkAttachment *api.NetworkAttachment) bool {
	return false
}

// IsServiceAllocated returns true iff [Inert.AllocateService] would return nil.
func (Inert) IsServiceAllocated(s *api.Service, flags ...func(*ServiceAllocationOpts)) bool {
	return (Inert{}).AllocateService(s) == nil
}

// IsTaskAllocated returns true iff [Inert.AllocateTask] would return nil.
func (Inert) IsTaskAllocated(t *api.Task) bool {
	return (Inert{}).AllocateTask(t) == nil
}
