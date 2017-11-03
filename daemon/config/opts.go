package config

import (
	"github.com/docker/swarmkit/api/genericresource"
	"github.com/moby/moby/api/types/swarm"
	"github.com/moby/moby/daemon/cluster/convert"
)

// ParseGenericResources parses and validates the specified string as a list of GenericResource
func ParseGenericResources(value string) ([]swarm.GenericResource, error) {
	if value == "" {
		return nil, nil
	}

	resources, err := genericresource.Parse(value)
	if err != nil {
		return nil, err
	}

	obj := convert.GenericResourcesFromGRPC(resources)
	return obj, nil
}
