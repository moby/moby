package node

import (
	"fmt"
	"strings"

	"github.com/docker/engine-api/types/swarm"
)

type nodeOptions struct {
	role         string
	membership   string
	availability string
}

func (opts *nodeOptions) ToNodeSpec() (swarm.NodeSpec, error) {
	var spec swarm.NodeSpec

	switch swarm.NodeRole(strings.ToLower(opts.role)) {
	case swarm.NodeRoleWorker:
		spec.Role = swarm.NodeRoleWorker
	case swarm.NodeRoleManager:
		spec.Role = swarm.NodeRoleManager
	case "":
	default:
		return swarm.NodeSpec{}, fmt.Errorf("invalid role %q, only worker and manager are supported", opts.role)
	}

	switch swarm.NodeMembership(strings.ToLower(opts.membership)) {
	case swarm.NodeMembershipAccepted:
		spec.Membership = swarm.NodeMembershipAccepted
	case "":
	default:
		return swarm.NodeSpec{}, fmt.Errorf("invalid membership %q, only accepted is supported", opts.membership)
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
