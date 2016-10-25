package node

import (
	"fmt"
	"strings"

	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/opts"
	runconfigopts "github.com/docker/docker/runconfig/opts"
)

type nodeOptions struct {
	annotations
	role         string
	availability string
}

type annotations struct {
	name   string
	labels opts.ListOpts
}

func newNodeOptions() *nodeOptions {
	return &nodeOptions{
		annotations: annotations{
			labels: opts.NewListOpts(nil),
		},
	}
}

func (opts *nodeOptions) ToNodeSpec() (swarm.NodeSpec, error) {
	var spec swarm.NodeSpec

	spec.Annotations.Name = opts.annotations.name
	spec.Annotations.Labels = runconfigopts.ConvertKVStringsToMap(opts.annotations.labels.GetAll())

	switch swarm.NodeRole(strings.ToLower(opts.role)) {
	case swarm.NodeRoleWorker:
		spec.Role = swarm.NodeRoleWorker
	case swarm.NodeRoleManager:
		spec.Role = swarm.NodeRoleManager
	case "":
	default:
		return swarm.NodeSpec{}, fmt.Errorf("invalid role %q, only worker and manager are supported", opts.role)
	}

	switch swarm.NodeAvailability(strings.ToLower(opts.availability)) {
	case swarm.NodeAvailabilityActive:
		spec.Availability = swarm.NodeAvailabilityActive
	case swarm.NodeAvailabilityPause:
		spec.Availability = swarm.NodeAvailabilityPause
	case swarm.NodeAvailabilityDrain:
		spec.Availability = swarm.NodeAvailabilityDrain
	case "":
	default:
		return swarm.NodeSpec{}, fmt.Errorf("invalid availability %q, only active, pause and drain are supported", opts.availability)
	}

	return spec, nil
}
