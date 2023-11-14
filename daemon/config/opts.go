package config // import "github.com/docker/docker/daemon/config"

import (
	"github.com/moby/swarmkit/v2/api/genericresource"

	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/daemon/cluster/convert"
)

// ParseGenericResources parses and validates the specified string as a list of GenericResource
func ParseGenericResources(value []string) ([]swarm.GenericResource, error) {
	if len(value) == 0 {
		return nil, nil
	}

	resources, err := genericresource.Parse(value)
	if err != nil {
		return nil, err
	}

	obj := convert.GenericResourcesFromGRPC(resources)
	return obj, nil
}
