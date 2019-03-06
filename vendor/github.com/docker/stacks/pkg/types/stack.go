package types

import (
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/stacks/pkg/compose/types"
)

// Stack represents a runtime instantiation of a Docker Compose based application
type Stack struct {
	Metadata
	Version
	Spec           StackSpec          `json:"spec"`
	StackResources StackResources     `json:"stack_resources"`
	Orchestrator   OrchestratorChoice `json:"orchestrator"`
	Status         StackStatus        `json:"stack_status"`

	// TODO - temporary (not in swagger)
	ID string
}

// StackCreateOptions is input to the Create operation for a Stack
type StackCreateOptions struct {
	EncodedRegistryAuth string
}

// StackUpdateOptions is input to the Update operation for a Stack
type StackUpdateOptions struct {
	EncodedRegistryAuth string
}

// StackListOptions is input to the List operation for a Stack
type StackListOptions struct {
	Filters filters.Args
}

// Version represents the internal object version.
type Version struct {
	Index uint64 `json:",omitempty"`
}

// StackCreate is input to the Create operation for a Stack
type StackCreate struct {
	Metadata
	Spec         StackSpec          `json:"spec"`
	Orchestrator OrchestratorChoice `json:"orchestrator"`
}

// Metadata contains metadata for a Stack.
type Metadata struct {
	Name string
}

// StackList is the output for Stack listing
type StackList struct {
	Items []Stack `json:"items"`
}

// StackSpec defines the desired state of Stack
type StackSpec struct {
	Services       types.Services                   `json:"services,omitempty"`
	Secrets        map[string]types.SecretConfig    `json:"secrets,omitempty"`
	Configs        map[string]types.ConfigObjConfig `json:"configs,omitempty"`
	Networks       map[string]types.NetworkConfig   `json:"networks,omitempty"`
	Volumes        map[string]types.VolumeConfig    `json:"volumes,omitempty"`
	StackImage     string                           `json:"stack_image,omitempty"`
	PropertyValues []string                         `json:"property_values,omitempty"`
	Collection     string                           `json:"collection,omitempty"`
}

// StackResources links to the running instances of the StackSpec
type StackResources struct {
	Services map[string]StackResource `json:"services,omitempty"`
	Configs  map[string]StackResource `json:"configs,omitempty"`
	Secrets  map[string]StackResource `json:"secrets,omitempty"`
	Networks map[string]StackResource `json:"networks,omitempty"`
	Volumes  map[string]StackResource `json:"volumes,omitempty"`
}

// StackResource contains a link to a single instance of the spec
// For example, when a Service is run on basic containers, the ID would
// contain the container ID.  When the Service is running on Swarm the ID would be
// a Swarm Service ID.  When mapped to kubernetes, it would map to a Deployment or
// DaemonSet ID.
type StackResource struct {
	Orchestrator OrchestratorChoice `json:"orchestrator"`
	Kind         string             `json:"kind"`
	ID           string             `json:"id"`
}

// StackStatus defines the observed state of Stack
type StackStatus struct {
	Message       string `json:"message"`
	Phase         string `json:"phase"`
	OverallHealth string `json:"overall_health"`
	// ServicesStatus contains the last known status of the service
	// The service name is the key in the map.
	ServicesStatus map[string]ServiceStatus `json:"services_status"`
	LastUpdated    string                   `json:"last_updated"`
}

// ServiceStatus represents the latest known status of a service
type ServiceStatus struct {
	// DesiredTasks represents the expected number of running tasks
	// given the current service spec settings, and number of nodes
	// in the cluster that satisfy those constraints.
	DesiredTasks uint64 `json:"desired_tasks"`
	RunningTasks uint64 `json:"running_tasks"`
}

// StackTaskList contains a summary of the underlying tasks that make up this Stack
type StackTaskList struct {
	CurrentTasks []StackTask `json:"current_tasks"`
	PastTasks    []StackTask `json:"past_tasks"`
}

// StackTask This contains a summary of the Stacks task
type StackTask struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Image        string `json:"image"`
	NodeID       string `json:"node_id"`
	DesiredState string `json:"desired_state"`
	CurrentState string `json:"current_state"`
	Err          string `json:"err"`
}

// OrchestratorChoice This field specifies which orchestrator the stack is deployed on.
type OrchestratorChoice string

const (
	// OrchestratorSwarm defines the OrchestratorChoice valud for Swarm
	OrchestratorSwarm = "swarm"

	// OrchestratorKubernetes defines the OrchestratorChoice valud for Kubernetes
	OrchestratorKubernetes = "kubernetes"

	// OrchestratorNone defines the OrchestratorChoice valud for no orchestrator (basic containers)
	OrchestratorNone = "none"
)

// ComposeInput carries one or more compose files for parsing by the server
type ComposeInput struct {
	ComposeFiles []string `json:"compose_files"`
}

// StackCreateResponse is the response type of the Create Stack
// operation.
type StackCreateResponse struct {
	ID string
}
