package swarm // import "github.com/docker/docker/api/server/router/swarm"

import (
	"context"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/backend"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/swarm"
)

// Backend abstracts a swarm manager.
type Backend interface {
	Init(req swarm.InitRequest) (string, error)
	Join(req swarm.JoinRequest) error
	Leave(ctx context.Context, force bool) error
	Inspect() (swarm.Swarm, error)
	Update(uint64, swarm.Spec, swarm.UpdateFlags) error
	GetUnlockKey() (string, error)
	UnlockSwarm(req swarm.UnlockRequest) error
	GetServices(types.ServiceListOptions) ([]swarm.Service, error)
	GetService(idOrName string, insertDefaults bool) (swarm.Service, error)
	CreateService(swarm.ServiceSpec, string, bool) (*swarm.ServiceCreateResponse, error)
	UpdateService(string, uint64, swarm.ServiceSpec, types.ServiceUpdateOptions, bool) (*swarm.ServiceUpdateResponse, error)
	RemoveService(string) error
	ServiceLogs(context.Context, *backend.LogSelector, *container.LogsOptions) (<-chan *backend.LogMessage, error)
	GetNodes(types.NodeListOptions) ([]swarm.Node, error)
	GetNode(string) (swarm.Node, error)
	UpdateNode(string, uint64, swarm.NodeSpec) error
	RemoveNode(string, bool) error
	GetTasks(types.TaskListOptions) ([]swarm.Task, error)
	GetTask(string) (swarm.Task, error)
	GetSecrets(opts types.SecretListOptions) ([]swarm.Secret, error)
	CreateSecret(s swarm.SecretSpec) (string, error)
	RemoveSecret(idOrName string) error
	GetSecret(id string) (swarm.Secret, error)
	UpdateSecret(idOrName string, version uint64, spec swarm.SecretSpec) error
	GetConfigs(opts types.ConfigListOptions) ([]swarm.Config, error)
	CreateConfig(s swarm.ConfigSpec) (string, error)
	RemoveConfig(id string) error
	GetConfig(id string) (swarm.Config, error)
	UpdateConfig(idOrName string, version uint64, spec swarm.ConfigSpec) error
}
