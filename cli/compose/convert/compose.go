package convert

import (
	"io/ioutil"
	"strings"

	"github.com/docker/docker/api/types"
	networktypes "github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/swarm"
	composetypes "github.com/docker/docker/cli/compose/types"
)

const (
	// LabelNamespace is the label used to track stack resources
	LabelNamespace = "com.docker.stack.namespace"
)

// Namespace mangles names by prepending the name
type Namespace struct {
	name string
}

// Scope prepends the namespace to a name
func (n Namespace) Scope(name string) string {
	return n.name + "_" + name
}

// Descope returns the name without the namespace prefix
func (n Namespace) Descope(name string) string {
	return strings.TrimPrefix(name, n.name+"_")
}

// Name returns the name of the namespace
func (n Namespace) Name() string {
	return n.name
}

// NewNamespace returns a new Namespace for scoping of names
func NewNamespace(name string) Namespace {
	return Namespace{name: name}
}

// AddStackLabel returns labels with the namespace label added
func AddStackLabel(namespace Namespace, labels map[string]string) map[string]string {
	if labels == nil {
		labels = make(map[string]string)
	}
	labels[LabelNamespace] = namespace.name
	return labels
}

type networkMap map[string]composetypes.NetworkConfig

// Networks from the compose-file type to the engine API type
func Networks(namespace Namespace, networks networkMap, servicesNetworks map[string]struct{}) (map[string]types.NetworkCreate, []string) {
	if networks == nil {
		networks = make(map[string]composetypes.NetworkConfig)
	}

	externalNetworks := []string{}
	result := make(map[string]types.NetworkCreate)
	for internalName := range servicesNetworks {
		network := networks[internalName]
		if network.External.External {
			externalNetworks = append(externalNetworks, network.External.Name)
			continue
		}

		createOpts := types.NetworkCreate{
			Labels:     AddStackLabel(namespace, network.Labels),
			Driver:     network.Driver,
			Options:    network.DriverOpts,
			Internal:   network.Internal,
			Attachable: network.Attachable,
		}

		if network.Ipam.Driver != "" || len(network.Ipam.Config) > 0 {
			createOpts.IPAM = &networktypes.IPAM{}
		}

		if network.Ipam.Driver != "" {
			createOpts.IPAM.Driver = network.Ipam.Driver
		}
		for _, ipamConfig := range network.Ipam.Config {
			config := networktypes.IPAMConfig{
				Subnet: ipamConfig.Subnet,
			}
			createOpts.IPAM.Config = append(createOpts.IPAM.Config, config)
		}
		result[internalName] = createOpts
	}

	return result, externalNetworks
}

// Secrets converts secrets from the Compose type to the engine API type
func Secrets(namespace Namespace, secrets map[string]composetypes.SecretConfig) ([]swarm.SecretSpec, error) {
	result := []swarm.SecretSpec{}
	for name, secret := range secrets {
		if secret.External.External {
			continue
		}

		data, err := ioutil.ReadFile(secret.File)
		if err != nil {
			return nil, err
		}

		result = append(result, swarm.SecretSpec{
			Annotations: swarm.Annotations{
				Name:   namespace.Scope(name),
				Labels: AddStackLabel(namespace, secret.Labels),
			},
			Data: data,
		})
	}
	return result, nil
}
