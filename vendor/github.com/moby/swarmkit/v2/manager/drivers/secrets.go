package drivers

import (
	"fmt"

	"github.com/docker/docker/pkg/plugingetter"
	"github.com/moby/swarmkit/v2/api"
	"github.com/moby/swarmkit/v2/api/naming"
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

// Get gets a secret from the secret provider. The function returns: the secret value;
// a bool indicating whether the value should be reused across different tasks (defaults to false);
// and an error if either the spec or task are nil, if calling the driver returns an error, or if
// the driver returns an error in the payload.
func (d *SecretDriver) Get(spec *api.SecretSpec, task *api.Task) ([]byte, bool, error) {
	if spec == nil {
		return nil, false, fmt.Errorf("secret spec is nil")
	}
	if task == nil {
		return nil, false, fmt.Errorf("task is nil")
	}

	var secretResp SecretsProviderResponse
	secretReq := &SecretsProviderRequest{
		SecretName:    spec.Annotations.Name,
		SecretLabels:  spec.Annotations.Labels,
		ServiceID:     task.ServiceID,
		ServiceName:   task.ServiceAnnotations.Name,
		ServiceLabels: task.ServiceAnnotations.Labels,
		TaskID:        task.ID,
		TaskName:      naming.Task(task),
		TaskImage:     task.Spec.GetContainer().Image,
		NodeID:        task.NodeID,
	}
	container := task.Spec.GetContainer()
	if container != nil {
		secretReq.ServiceHostname = container.Hostname
	}

	if task.Endpoint != nil && task.Endpoint.Spec != nil {
		secretReq.ServiceEndpointSpec = &EndpointSpec{
			Mode: int32(task.Endpoint.Spec.Mode),
		}
		for _, p := range task.Endpoint.Spec.Ports {
			if p == nil {
				continue
			}
			secretReq.ServiceEndpointSpec.Ports =
				append(secretReq.ServiceEndpointSpec.Ports,
					PortConfig{
						Name:          p.Name,
						Protocol:      int32(p.Protocol),
						PublishedPort: p.PublishedPort,
						TargetPort:    p.TargetPort,
						PublishMode:   int32(p.PublishMode),
					})
		}
	}

	err := d.plugin.Client().Call(SecretsProviderAPI, secretReq, &secretResp)
	if err != nil {
		return nil, false, err
	}
	if secretResp.Err != "" {
		return nil, secretResp.DoNotReuse, fmt.Errorf(secretResp.Err)
	}
	// Assign the secret value
	return secretResp.Value, secretResp.DoNotReuse, nil
}

// SecretsProviderRequest is the secrets provider request.
type SecretsProviderRequest struct {
	SecretName          string            `json:",omitempty"` // SecretName is the name of the secret to request from the plugin
	SecretLabels        map[string]string `json:",omitempty"` // SecretLabels capture environment names and other metadata pertaining to the secret
	ServiceHostname     string            `json:",omitempty"` // ServiceHostname is the hostname of the service, can be used for x509 certificate
	ServiceID           string            `json:",omitempty"` // ServiceID is the name of the service that requested the secret
	ServiceName         string            `json:",omitempty"` // ServiceName is the name of the service that requested the secret
	ServiceLabels       map[string]string `json:",omitempty"` // ServiceLabels capture environment names and other metadata pertaining to the service
	TaskID              string            `json:",omitempty"` // TaskID is the ID of the task that the secret will be assigned to
	TaskName            string            `json:",omitempty"` // TaskName is the name of the task that the secret will be assigned to
	TaskImage           string            `json:",omitempty"` // TaskName is the image of the task that the secret will be assigned to
	NodeID              string            `json:",omitempty"` // NodeID is the ID of the node that the task will be executed on
	ServiceEndpointSpec *EndpointSpec     `json:",omitempty"` // ServiceEndpointSpec holds the specification for endpoints
}

// SecretsProviderResponse is the secrets provider response.
type SecretsProviderResponse struct {
	Value []byte `json:",omitempty"` // Value is the value of the secret
	Err   string `json:",omitempty"` // Err is the error response of the plugin

	// DoNotReuse indicates that the secret returned from this request should
	// only be used for one task, and any further tasks should call the secret
	// driver again.
	DoNotReuse bool `json:",omitempty"`
}

// EndpointSpec represents the spec of an endpoint.
type EndpointSpec struct {
	Mode  int32        `json:",omitempty"`
	Ports []PortConfig `json:",omitempty"`
}

// PortConfig represents the config of a port.
type PortConfig struct {
	Name     string `json:",omitempty"`
	Protocol int32  `json:",omitempty"`
	// TargetPort is the port inside the container
	TargetPort uint32 `json:",omitempty"`
	// PublishedPort is the port on the swarm hosts
	PublishedPort uint32 `json:",omitempty"`
	// PublishMode is the mode in which port is published
	PublishMode int32 `json:",omitempty"`
}
