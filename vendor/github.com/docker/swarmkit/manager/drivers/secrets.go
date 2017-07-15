package drivers

import (
	"fmt"

	"github.com/docker/docker/pkg/plugingetter"
	"github.com/docker/swarmkit/api"
)

const (
	// SecretsProviderAPI is the endpoint for fetching secrets from plugins
	SecretsProviderAPI = "/SecretProvider.GetSecret"

	// SecretsProviderCapability is the secrets provider plugin capability identification
	SecretsProviderCapability = "secretprovider"
)

// SecretDriver provides secrets from different stores
type SecretDriver struct {
	plugin plugingetter.CompatPlugin
}

// NewSecretDriver creates a new driver that provides third party secrets
func NewSecretDriver(plugin plugingetter.CompatPlugin) *SecretDriver {
	return &SecretDriver{plugin: plugin}
}

// Get gets a secret from the secret provider
func (d *SecretDriver) Get(spec *api.SecretSpec) ([]byte, error) {
	if spec == nil {
		return nil, fmt.Errorf("spec is nil")
	}
	var secretResp SecretsProviderResponse
	secretReq := &SecretsProviderRequest{Name: spec.Annotations.Name}
	err := d.plugin.Client().Call(SecretsProviderAPI, secretReq, &secretResp)
	if err != nil {
		return nil, err
	}
	if secretResp.Err != "" {
		return nil, fmt.Errorf(secretResp.Err)
	}
	// Assign the secret value
	return []byte(secretResp.Value), nil
}

// SecretsProviderRequest is the secrets provider request.
type SecretsProviderRequest struct {
	Name string `json:"name"` // Name is the name of the secret plugin
}

// SecretsProviderResponse is the secrets provider response.
type SecretsProviderResponse struct {
	Value string `json:"value"` // Value is the value of the secret
	Err   string `json:"err"`   // Err is the error response of the plugin
}
