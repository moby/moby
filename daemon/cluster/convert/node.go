package convert

import (
	"fmt"
	"strings"

	types "github.com/docker/engine-api/types/swarm"
	swarmapi "github.com/docker/swarmkit/api"
	"github.com/docker/swarmkit/protobuf/ptypes"
)

// NodeFromGRPC converts a grpc Node to a Node.
func NodeFromGRPC(n swarmapi.Node) types.Node {
	node := types.Node{
		ID: n.ID,
		Spec: types.NodeSpec{
			Role:         types.NodeRole(n.Spec.Role.String()),
			Membership:   types.NodeMembership(n.Spec.Membership.String()),
			Availability: types.NodeAvailability(n.Spec.Availability.String()),
		},
		Status: types.NodeStatus{
			State:   types.NodeState(n.Status.State.String()),
			Message: n.Status.Message,
		},
	}

	// Meta
	node.Version.Index = n.Meta.Version.Index
	node.CreatedAt, _ = ptypes.Timestamp(n.Meta.CreatedAt)
	node.UpdatedAt, _ = ptypes.Timestamp(n.Meta.UpdatedAt)

	//Annotations
	node.Spec.Name = n.Spec.Annotations.Name
	node.Spec.Labels = n.Spec.Annotations.Labels

	//Description
	if n.Description != nil {
		node.Description.Hostname = n.Description.Hostname
		if n.Description.Platform != nil {
			node.Description.Platform.Architecture = n.Description.Platform.Architecture
			node.Description.Platform.OS = n.Description.Platform.OS
		}
		if n.Description.Resources != nil {
			node.Description.Resources.NanoCPUs = n.Description.Resources.NanoCPUs
			node.Description.Resources.MemoryBytes = n.Description.Resources.MemoryBytes
		}
		if n.Description.Engine != nil {
			node.Description.Engine.EngineVersion = n.Description.Engine.EngineVersion
			node.Description.Engine.Labels = n.Description.Engine.Labels
			for _, plugin := range n.Description.Engine.Plugins {
				node.Description.Engine.Plugins = append(node.Description.Engine.Plugins, types.PluginDescription{Type: plugin.Type, Name: plugin.Name})
			}
		}
	}

	//Manager
	if n.Manager != nil {
		node.Manager = &types.Manager{
			Raft: types.RaftMember{
				RaftID: n.Manager.Raft.RaftID,
				Addr:   n.Manager.Raft.Addr,
				Status: types.RaftMemberStatus{
					Leader:       n.Manager.Raft.Status.Leader,
					Reachability: types.Reachability(n.Manager.Raft.Status.Reachability.String()),
					Message:      n.Manager.Raft.Status.Message,
				},
			},
		}
	}

	return node
}

// NodeSpecToGRPC converts a Node to a grpc NodeSpec.
func NodeSpecToGRPC(n types.Node) (swarmapi.NodeSpec, error) {
	spec := swarmapi.NodeSpec{
		Annotations: swarmapi.Annotations{
			Name:   n.Spec.Name,
			Labels: n.Spec.Labels,
		},
	}
	if role, ok := swarmapi.NodeRole_value[strings.ToUpper(string(n.Spec.Role))]; ok {
		spec.Role = swarmapi.NodeRole(role)
	} else {
		return swarmapi.NodeSpec{}, fmt.Errorf("invalid Role: %q", n.Spec.Role)
	}

	if membership, ok := swarmapi.NodeSpec_Membership_value[strings.ToUpper(string(n.Spec.Membership))]; ok {
		spec.Membership = swarmapi.NodeSpec_Membership(membership)
	} else {
		return swarmapi.NodeSpec{}, fmt.Errorf("invalid Membership: %q", n.Spec.Membership)
	}

	if availability, ok := swarmapi.NodeSpec_Availability_value[strings.ToUpper(string(n.Spec.Availability))]; ok {
		spec.Availability = swarmapi.NodeSpec_Availability(availability)
	} else {
		return swarmapi.NodeSpec{}, fmt.Errorf("invalid Availability: %q", n.Spec.Availability)
	}

	return spec, nil
}
