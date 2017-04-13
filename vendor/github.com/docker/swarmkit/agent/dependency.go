package agent

import (
	"github.com/docker/swarmkit/agent/configs"
	"github.com/docker/swarmkit/agent/exec"
	"github.com/docker/swarmkit/agent/secrets"
	"github.com/docker/swarmkit/api"
)

type dependencyManager struct {
	secrets exec.SecretsManager
	configs exec.ConfigsManager
}

// NewDependencyManager creates a dependency manager object that wraps
// objects which provide access to various dependency types.
func NewDependencyManager() exec.DependencyManager {
	return &dependencyManager{
		secrets: secrets.NewManager(),
		configs: configs.NewManager(),
	}
}

func (d *dependencyManager) Secrets() exec.SecretsManager {
	return d.secrets
}

func (d *dependencyManager) Configs() exec.ConfigsManager {
	return d.configs
}

type dependencyGetter struct {
	secrets exec.SecretGetter
	configs exec.ConfigGetter
}

func (d *dependencyGetter) Secrets() exec.SecretGetter {
	return d.secrets
}

func (d *dependencyGetter) Configs() exec.ConfigGetter {
	return d.configs
}

// Restrict provides getters that only allows access to the dependencies
// referenced by the task.
func Restrict(dependencies exec.DependencyManager, t *api.Task) exec.DependencyGetter {
	return &dependencyGetter{
		secrets: secrets.Restrict(dependencies.Secrets(), t),
		configs: configs.Restrict(dependencies.Configs(), t),
	}
}
