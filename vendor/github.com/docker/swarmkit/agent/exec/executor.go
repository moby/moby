package exec

import (
	"github.com/docker/swarmkit/api"
	"golang.org/x/net/context"
)

// Executor provides controllers for tasks.
type Executor interface {
	// Describe returns the underlying node description.
	Describe(ctx context.Context) (*api.NodeDescription, error)

	// Configure uses the node object state to propagate node
	// state to the underlying executor.
	Configure(ctx context.Context, node *api.Node) error

	// Controller provides a controller for the given task.
	Controller(t *api.Task) (Controller, error)

	// SetNetworkBootstrapKeys passes the symmetric keys from the
	// manager to the executor.
	SetNetworkBootstrapKeys([]*api.EncryptionKey) error
}

// SecretsProvider is implemented by objects that can store secrets, typically
// an executor.
type SecretsProvider interface {
	Secrets() SecretsManager
}

// ConfigsProvider is implemented by objects that can store configs,
// typically an executor.
type ConfigsProvider interface {
	Configs() ConfigsManager
}

// DependencyManager is a meta-object that can keep track of typed objects
// such as secrets and configs.
type DependencyManager interface {
	SecretsProvider
	ConfigsProvider
}

// DependencyGetter is a meta-object that can provide access to typed objects
// such as secrets and configs.
type DependencyGetter interface {
	Secrets() SecretGetter
	Configs() ConfigGetter
}

// SecretGetter contains secret data necessary for the Controller.
type SecretGetter interface {
	// Get returns the the secret with a specific secret ID, if available.
	// When the secret is not available, the return will be nil.
	Get(secretID string) (*api.Secret, error)
}

// SecretsManager is the interface for secret storage and updates.
type SecretsManager interface {
	SecretGetter

	Add(secrets ...api.Secret) // add one or more secrets
	Remove(secrets []string)   // remove the secrets by ID
	Reset()                    // remove all secrets
}

// ConfigGetter contains config data necessary for the Controller.
type ConfigGetter interface {
	// Get returns the the config with a specific config ID, if available.
	// When the config is not available, the return will be nil.
	Get(configID string) (*api.Config, error)
}

// ConfigsManager is the interface for config storage and updates.
type ConfigsManager interface {
	ConfigGetter

	Add(configs ...api.Config) // add one or more configs
	Remove(configs []string)   // remove the configs by ID
	Reset()                    // remove all configs
}
