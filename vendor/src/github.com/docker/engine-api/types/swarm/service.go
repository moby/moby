package swarm

import "time"

// Service represents a service.
type Service struct {
	ID string
	Meta
	Spec     ServiceSpec `json:",omitempty"`
	Endpoint Endpoint    `json:",omitempty"`
}

// ServiceSpec represents the spec of a service.
type ServiceSpec struct {
	Annotations
	TaskSpec TaskSpec    `json:",omitempty"`
	Mode     ServiceMode `json:",omitempty"`
	// TODO(aluzzardi): Update or UpdateConfig?
	UpdateConfig *UpdateConfig             `json:",omitempty"`
	Networks     []NetworkAttachmentConfig `json:",omitempty"`
	// TODO(aluzzardi): Endpoint, EndpointSpec?
	EndpointSpec *EndpointSpec `json:",omitempty"`
}

// ServiceMode represents the mode of a service.
type ServiceMode struct {
	Replicated *ReplicatedService `json:",omitempty"`
	Global     *GlobalService     `json:",omitempty"`
}

// ReplicatedService is a kind of ServiceMode.
type ReplicatedService struct {
	Instances *uint64 `json:",omitempty"`
}

// GlobalService is a kind of ServiceMode.
type GlobalService struct {
}

// UpdateConfig represents the update configuration.
type UpdateConfig struct {
	Parallelism uint64        `json:",omitempty"`
	Delay       time.Duration `json:",omitempty"`
}
