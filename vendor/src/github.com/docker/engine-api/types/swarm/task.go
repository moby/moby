package swarm

import "time"

const (
	// TaskStateNew NEW
	TaskStateNew TaskState = "NEW"
	// TaskStateAllocated ALLOCATED
	TaskStateAllocated TaskState = "ALLOCATED"
	// TaskStatePending PENDING
	TaskStatePending TaskState = "PENDING"
	// TaskStateAssigned ASSIGNED
	TaskStateAssigned TaskState = "ASSIGNED"
	// TaskStateAccepted ACCEPTED
	TaskStateAccepted TaskState = "ACCEPTED"
	// TaskStatePreparing PREPARING
	TaskStatePreparing TaskState = "PREPARING"
	// TaskStateReady READY
	TaskStateReady TaskState = "READY"
	// TaskStateStarting STARTING
	TaskStateStarting TaskState = "STARTING"
	// TaskStateRunning RUNNING
	TaskStateRunning TaskState = "RUNNING"
	// TaskStateComplete COMPLETE
	TaskStateComplete TaskState = "COMPLETE"
	// TaskStateShutdown SHUTDOWN
	TaskStateShutdown TaskState = "SHUTDOWN"
	// TaskStateFailed FAILED
	TaskStateFailed TaskState = "FAILED"
	// TaskStateRejected REJECTED
	TaskStateRejected TaskState = "REJECTED"
)

// TaskState represents the state of a task.
type TaskState string

// Task represents a task.
type Task struct {
	ID string
	Meta

	Spec                TaskSpec            `json:",omitempty"`
	ServiceID           string              `json:",omitempty"`
	Instance            int                 `json:",omitempty"`
	NodeID              string              `json:",omitempty"`
	Status              TaskStatus          `json:",omitempty"`
	DesiredState        TaskState           `json:",omitempty"`
	NetworksAttachments []NetworkAttachment `json:",omitempty"`
}

// TaskSpec represents the spec of a task.
type TaskSpec struct {
	ContainerSpec ContainerSpec         `json:",omitempty"`
	Resources     *ResourceRequirements `json:",omitempty"`
	RestartPolicy *RestartPolicy        `json:",omitempty"`
	Placement     *Placement            `json:",omitempty"`
}

// Resources represents resources (CPU/Memory).
type Resources struct {
	NanoCPUs    int64 `json:",omitempty"`
	MemoryBytes int64 `json:",omitempty"`
}

// ResourceRequirements represents resources requirements.
type ResourceRequirements struct {
	Limits       *Resources `json:",omitempty"`
	Reservations *Resources `json:",omitempty"`
}

// Placement represents orchestration parameters.
type Placement struct {
	Constraints []string `json:",omitempty"`
}

// RestartPolicy represents the restart policy.
type RestartPolicy struct {
	Condition   RestartPolicyCondition `json:",omitempty"`
	Delay       *time.Duration         `json:",omitempty"`
	MaxAttempts *uint64                `json:",omitempty"`
	Window      *time.Duration         `json:",omitempty"`
}

const (
	// RestartPolicyConditionNone NONE
	RestartPolicyConditionNone RestartPolicyCondition = "NONE"
	// RestartPolicyConditionOnFailure ON_FAILURE
	RestartPolicyConditionOnFailure RestartPolicyCondition = "ON_FAILURE"
	// RestartPolicyConditionAny ANY
	RestartPolicyConditionAny RestartPolicyCondition = "ANY"
)

// RestartPolicyCondition represents when to restart.
type RestartPolicyCondition string

// TaskStatus represents the status of a task.
type TaskStatus struct {
	Timestamp       time.Time       `json:",omitempty"`
	State           TaskState       `json:",omitempty"`
	Message         string          `json:",omitempty"`
	Err             string          `json:",omitempty"`
	ContainerStatus ContainerStatus `json:",omitempty"`
}

// ContainerStatus represents the status of a container.
type ContainerStatus struct {
	ContainerID string `json:",omitempty"`
	PID         int    `json:",omitempty"`
	ExitCode    int    `json:",omitempty"`
}
