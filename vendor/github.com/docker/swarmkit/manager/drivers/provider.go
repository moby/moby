package drivers

import (
	"fmt"

	"github.com/docker/docker/pkg/plugingetter"
	"github.com/docker/swarmkit/api"
)

// DriverProvider provides external drivers
type DriverProvider struct {
	pluginGetter plugingetter.PluginGetter
}

// New returns a new driver provider
func New(pluginGetter plugingetter.PluginGetter) *DriverProvider {
	return &DriverProvider{pluginGetter: pluginGetter}
}

// NewSecretDriver creates a new driver for fetching secrets
func (m *DriverProvider) NewSecretDriver(driver *api.Driver) (*SecretDriver, error) {
	if m.pluginGetter == nil {
		return nil, fmt.Errorf("plugin getter is nil")
	}
	if driver == nil && driver.Name == "" {
		return nil, fmt.Errorf("driver specification is nil")
	}
	// Search for the specified plugin
	plugin, err := m.pluginGetter.Get(driver.Name, SecretsProviderCapability, plugingetter.Lookup)
	if err != nil {
		return nil, err
	}
	return NewSecretDriver(plugin), nil
}
