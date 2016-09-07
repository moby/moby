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
	GetServices(basictypes.ServiceListOptions) ([]types.Service, error)
	GetService(string) (types.Service, error)
	CreateService(types.ServiceSpec, string) (string, error)
	UpdateService(string, uint64, types.ServiceSpec, string) error
	RemoveService(string) error
	GetNodes(basictypes.NodeListOptions) ([]types.Node, error)
	GetNode(string) (types.Node, error)
	UpdateNode(string, uint64, types.NodeSpec) error
	RemoveNode(string, bool) error
	GetTasks(basictypes.TaskListOptions) ([]types.Task, error)
	GetTask(string) (types.Task, error)
}
