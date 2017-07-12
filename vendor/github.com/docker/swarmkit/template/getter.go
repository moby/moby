package template

import (
	"github.com/docker/swarmkit/agent/exec"
	"github.com/docker/swarmkit/api"
	"github.com/pkg/errors"
)

type templatedSecretGetter struct {
	dependencies exec.DependencyGetter
	t            *api.Task
	node         *api.NodeDescription
}

// NewTemplatedSecretGetter returns a SecretGetter that evaluates templates.
func NewTemplatedSecretGetter(dependencies exec.DependencyGetter, t *api.Task, node *api.NodeDescription) exec.SecretGetter {
	return templatedSecretGetter{dependencies: dependencies, t: t, node: node}
}

func (t templatedSecretGetter) Get(secretID string) (*api.Secret, error) {
	if t.dependencies == nil {
		return nil, errors.New("no secret provider available")
	}

	secrets := t.dependencies.Secrets()
	if secrets == nil {
		return nil, errors.New("no secret provider available")
	}

	secret, err := secrets.Get(secretID)
	if err != nil {
		return secret, err
	}

	newSpec, err := ExpandSecretSpec(secret, t.node, t.t, t.dependencies)
	if err != nil {
		return secret, errors.Wrapf(err, "failed to expand templated secret %s", secretID)
	}

	secretCopy := *secret
	secretCopy.Spec = *newSpec
	return &secretCopy, nil
}

type templatedConfigGetter struct {
	dependencies exec.DependencyGetter
	t            *api.Task
	node         *api.NodeDescription
}

// NewTemplatedConfigGetter returns a ConfigGetter that evaluates templates.
func NewTemplatedConfigGetter(dependencies exec.DependencyGetter, t *api.Task, node *api.NodeDescription) exec.ConfigGetter {
	return templatedConfigGetter{dependencies: dependencies, t: t, node: node}
}

func (t templatedConfigGetter) Get(configID string) (*api.Config, error) {
	if t.dependencies == nil {
		return nil, errors.New("no config provider available")
	}

	configs := t.dependencies.Configs()
	if configs == nil {
		return nil, errors.New("no config provider available")
	}

	config, err := configs.Get(configID)
	if err != nil {
		return config, err
	}

	newSpec, err := ExpandConfigSpec(config, t.node, t.t, t.dependencies)
	if err != nil {
		return config, errors.Wrapf(err, "failed to expand templated config %s", configID)
	}

	configCopy := *config
	configCopy.Spec = *newSpec
	return &configCopy, nil
}

type templatedDependencyGetter struct {
	secrets exec.SecretGetter
	configs exec.ConfigGetter
}

// NewTemplatedDependencyGetter returns a DependencyGetter that evaluates templates.
func NewTemplatedDependencyGetter(dependencies exec.DependencyGetter, t *api.Task, node *api.NodeDescription) exec.DependencyGetter {
	return templatedDependencyGetter{
		secrets: NewTemplatedSecretGetter(dependencies, t, node),
		configs: NewTemplatedConfigGetter(dependencies, t, node),
	}
}

func (t templatedDependencyGetter) Secrets() exec.SecretGetter {
	return t.secrets
}

func (t templatedDependencyGetter) Configs() exec.ConfigGetter {
	return t.configs
}
