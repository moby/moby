package swarm

import (
	"context"

	"github.com/moby/moby/api/types/swarm"
	"github.com/moby/moby/v2/daemon/server/backend"
	"github.com/moby/moby/v2/daemon/server/swarmbackend"
)

// Backend abstracts a swarm manager.
type Backend interface {
	Init(ctx context.Context, req swarm.InitRequest) (string, error)
	Join(ctx context.Context, req swarm.JoinRequest) error
	Leave(ctx context.Context, force bool) error
	Inspect(context.Context) (swarm.Swarm, error)
	Update(context.Context, uint64, swarm.Spec, swarmbackend.UpdateFlags) error
	GetUnlockKey(context.Context) (string, error)
	UnlockSwarm(context.Context, swarm.UnlockRequest) error
	GetServices(context.Context, swarmbackend.ServiceListOptions) ([]swarm.Service, error)
	GetService(ctx context.Context, idOrName string, insertDefaults bool) (swarm.Service, error)
	CreateService(context.Context, swarm.ServiceSpec, string, bool) (*swarm.ServiceCreateResponse, error)
	UpdateService(context.Context, string, uint64, swarm.ServiceSpec, swarmbackend.ServiceUpdateOptions, bool) (*swarm.ServiceUpdateResponse, error)
	RemoveService(context.Context, string) error
	ServiceLogs(context.Context, *backend.LogSelector, *backend.ContainerLogsOptions) (<-chan *backend.LogMessage, error)
	GetNodes(context.Context, swarmbackend.NodeListOptions) ([]swarm.Node, error)
	GetNode(context.Context, string) (swarm.Node, error)
	UpdateNode(context.Context, string, uint64, swarm.NodeSpec) error
	RemoveNode(context.Context, string, bool) error
	GetTasks(context.Context, swarmbackend.TaskListOptions) ([]swarm.Task, error)
	GetTask(context.Context, string) (swarm.Task, error)
	GetSecrets(context.Context, swarmbackend.SecretListOptions) ([]swarm.Secret, error)
	CreateSecret(context.Context, swarm.SecretSpec) (string, error)
	RemoveSecret(context.Context, string) error
	GetSecret(context.Context, string) (swarm.Secret, error)
	UpdateSecret(ctx context.Context, idOrName string, version uint64, spec swarm.SecretSpec) error
	GetConfigs(context.Context, swarmbackend.ConfigListOptions) ([]swarm.Config, error)
	CreateConfig(context.Context, swarm.ConfigSpec) (string, error)
	RemoveConfig(context.Context, string) error
	GetConfig(context.Context, string) (swarm.Config, error)
	UpdateConfig(ctx context.Context, idOrName string, version uint64, spec swarm.ConfigSpec) error
}
