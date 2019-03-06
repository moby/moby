package interfaces

import (
	"time"

	"github.com/docker/docker/api/server/router/network"
	dockerTypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/swarm"

	"github.com/docker/stacks/pkg/types"
)

// StacksBackend is the backend handler for Stacks within the engine.
// It is consumed by the API handlers, and by the Reconciler.
type StacksBackend interface {
	CreateStack(types.StackCreate) (types.StackCreateResponse, error)
	GetStack(id string) (types.Stack, error)
	ListStacks() ([]types.Stack, error)
	UpdateStack(id string, spec types.StackSpec, version uint64) error
	DeleteStack(id string) error

	// The following operations are only used by the Reconciler and not
	// exposed via the Stacks API.
	GetSwarmStack(id string) (SwarmStack, error)
	ListSwarmStacks() ([]SwarmStack, error)

	ParseComposeInput(input types.ComposeInput) (*types.StackCreate, error)
}

// SwarmResourceBackend is a subset of the swarm.Backend interface,
// combined with the network.ClusterBackend interface. It includes all
// methods required to validate, provision and update manipulate Swarm
// stacks and their referenced resources.
type SwarmResourceBackend interface {
	network.ClusterBackend

	// Info isn't actually in the swarm.Backend interface, but it is defined on
	// the Cluster object, which provides the rest of the implementation
	Info() swarm.Info

	// The following methods are part of the swarm.Backend interface
	GetNode(id string) (swarm.Node, error)
	GetServices(dockerTypes.ServiceListOptions) ([]swarm.Service, error)
	GetService(idOrName string, insertDefaults bool) (swarm.Service, error)
	CreateService(swarm.ServiceSpec, string, bool) (*dockerTypes.ServiceCreateResponse, error)
	UpdateService(string, uint64, swarm.ServiceSpec, dockerTypes.ServiceUpdateOptions, bool) (*dockerTypes.ServiceUpdateResponse, error)
	RemoveService(string) error
	GetTasks(dockerTypes.TaskListOptions) ([]swarm.Task, error)
	GetTask(string) (swarm.Task, error)
	GetSecrets(opts dockerTypes.SecretListOptions) ([]swarm.Secret, error)
	CreateSecret(s swarm.SecretSpec) (string, error)
	RemoveSecret(idOrName string) error
	GetSecret(id string) (swarm.Secret, error)
	UpdateSecret(idOrName string, version uint64, spec swarm.SecretSpec) error
	GetConfigs(opts dockerTypes.ConfigListOptions) ([]swarm.Config, error)
	CreateConfig(s swarm.ConfigSpec) (string, error)
	RemoveConfig(id string) error
	GetConfig(id string) (swarm.Config, error)
	UpdateConfig(idOrName string, version uint64, spec swarm.ConfigSpec) error
}

// BackendClient is the full interface used by the Stacks Reconciler to
// consume Docker Events and act upon swarmkit resources. In the engine
// runtime, it is implemented directly by the docker/daemon.Daemon
// object. In the standalone test runtime, the BackendAPIClientShim
// allows a normal engine API to be used in its place.
type BackendClient interface {
	StacksBackend

	SwarmResourceBackend

	// SubscribeToEvents and UnsubscribeFromEvents are part of the
	// system.Backend interface.
	SubscribeToEvents(since, until time.Time, ef filters.Args) ([]events.Message, chan interface{})
	UnsubscribeFromEvents(chan interface{})
}

// StackStore defines an interface to an arbitrary store which is able
// to perform CRUD operations for all objects required by the Stacks
// Controller.
type StackStore interface {
	AddStack(types.Stack, SwarmStack) (string, error)
	UpdateStack(string, types.StackSpec, SwarmStackSpec, uint64) error
	DeleteStack(string) error

	GetStack(id string) (types.Stack, error)
	GetSwarmStack(id string) (SwarmStack, error)

	ListStacks() ([]types.Stack, error)
	ListSwarmStacks() ([]SwarmStack, error)
}
