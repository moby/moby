package swarm

import (
	basictypes "github.com/docker/docker/api/types"
	types "github.com/docker/docker/api/types/swarm"
)

// Backend abstracts an swarm commands manager.
type Backend interface {
	Init(req types.InitRequest) (string, error)
	Join(req types.JoinRequest) error
	Leave(force bool) error
	Inspect() (types.Swarm, error)
	Update(uint64, types.Spec, types.UpdateFlags) error
	GetUnlockKey() (string, error)
	UnlockSwarm(req types.UnlockRequest) error
	GetServices(basictypes.ServiceListOptions) ([]types.Service, error)
	GetService(string) (types.Service, error)
	CreateService(types.ServiceSpec, string) (string, error)
	UpdateService(string, uint64, types.ServiceSpec, string, string) error
	RemoveService(string) error
	GetNodes(basictypes.NodeListOptions) ([]types.Node, error)
	GetNode(string) (types.Node, error)
	UpdateNode(string, uint64, types.NodeSpec) error
	RemoveNode(string, bool) error
	GetTasks(basictypes.TaskListOptions) ([]types.Task, error)
	GetTask(string) (types.Task, error)
	GetSecrets(opts basictypes.SecretListOptions) ([]types.Secret, error)
	CreateSecret(s types.SecretSpec) (string, error)
	RemoveSecret(id string) error
	GetSecret(id string) (types.Secret, error)
	UpdateSecret(id string, version uint64, spec types.SecretSpec) error
}
