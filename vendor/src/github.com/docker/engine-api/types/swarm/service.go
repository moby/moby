package swarm

import "time"

// Service represents a service.
type Service struct {
	ID string
	Meta
	Spec         ServiceSpec  `json:",omitempty"`
	Endpoint     Endpoint     `json:",omitempty"`
	UpdateStatus UpdateStatus `json:",omitempty"`
}

// ServiceSpec represents the spec of a service.
type ServiceSpec struct {
	Annotations

	// TaskTemplate defines how the service should construct new tasks when
	// orchestrating this service.
	TaskTemplate TaskSpec      `json:",omitempty"`
	Mode         ServiceMode   `json:",omitempty"`
	UpdateConfig *UpdateConfig `json:",omitempty"`

	// Networks field in ServiceSpec is being deprecated. Users of
	// engine-api should start using the same field in
	// TaskSpec. This field will be removed in future releases.
	Networks     []NetworkAttachmentConfig `json:",omitempty"`
	EndpointSpec *EndpointSpec             `json:",omitempty"`
}

// ServiceMode represents the mode of a service.
type ServiceMode struct {
	Replicated *ReplicatedService `json:",omitempty"`
	Global     *GlobalService     `json:",omitempty"`
}

// UpdateState is the state of a service update.
type UpdateState string

const (
	// UpdateStateUpdating is the updating state.
	UpdateStateUpdating UpdateState = "updating"
	// UpdateStatePaused is the paused state.
	UpdateStatePaused UpdateState = "paused"
	// UpdateStateCompleted is the completed state.
	UpdateStateCompleted UpdateState = "completed"
)

// UpdateStatus reports the status of a service update.
type UpdateStatus struct {
	State       UpdateState `json:",omitempty"`
	StartedAt   time.Time   `json:",omitempty"`
	CompletedAt time.Time   `json:",omitempty"`
	Message     string      `json:",omitempty"`
}

// ReplicatedService is a kind of ServiceMode.
type ReplicatedService struct {
	Replicas *uint64 `json:",omitempty"`
}

// GlobalService is a kind of ServiceMode.
type GlobalService struct{}

const (
	// UpdateFailureActionPause PAUSE
	UpdateFailureActionPause = "pause"
	// UpdateFailureActionContinue CONTINUE
	UpdateFailureActionContinue = "continue"
)

// UpdateConfig represents the update configuration.
type UpdateConfig struct {
	Parallelism   uint64        `json:",omitempty"`
	Delay         time.Duration `json:",omitempty"`
	FailureAction string        `json:",omitempty"`
}
