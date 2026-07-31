package convert

import (
	"fmt"
	"strings"

	types "github.com/moby/moby/api/types/swarm"
	swarmapi "github.com/moby/swarmkit/v2/api"
)

// NodeFromGRPC converts a grpc Node to a Node.
func NodeFromGRPC(n *swarmapi.Node) types.Node {
	node := types.Node{
		ID: n.Id,
		Spec: types.NodeSpec{
			Role:         types.NodeRole(strings.ToLower(n.Spec.DesiredRole.String())),
			Availability: types.NodeAvailability(strings.ToLower(n.Spec.Availability.String())),
		},
		Status: types.NodeStatus{
			State:   types.NodeState(strings.ToLower(n.Status.State.String())),
			Message: n.Status.Message,
			Addr:    n.Status.Addr,
		},
	}

	// Meta
	node.Version.Index = n.Meta.Version.Index
	node.CreatedAt = n.Meta.CreatedAt.AsTime()
	node.UpdatedAt = n.Meta.UpdatedAt.AsTime()

	// Annotations
	node.Spec.Annotations = annotationsFromGRPC(n.Spec.Annotations)

	// Description
	if n.Description != nil {
		node.Description.Hostname = n.Description.Hostname
		if n.Description.Platform != nil {
			node.Description.Platform.Architecture = n.Description.Platform.Architecture
			node.Description.Platform.OS = n.Description.Platform.Os
		}
		if n.Description.Resources != nil {
			node.Description.Resources.NanoCPUs = n.Description.Resources.NanoCpus
			node.Description.Resources.MemoryBytes = n.Description.Resources.MemoryBytes
			node.Description.Resources.GenericResources = GenericResourcesFromGRPC(n.Description.Resources.Generic)
		}
		if n.Description.Engine != nil {
			node.Description.Engine.EngineVersion = n.Description.Engine.EngineVersion
			node.Description.Engine.Labels = n.Description.Engine.Labels
			for _, plugin := range n.Description.Engine.Plugins {
				node.Description.Engine.Plugins = append(node.Description.Engine.Plugins, types.PluginDescription{Type: plugin.Type, Name: plugin.Name})
			}
		}
		if n.Description.TlsInfo != nil {
			node.Description.TLSInfo.TrustRoot = string(n.Description.TlsInfo.TrustRoot)
			node.Description.TLSInfo.CertIssuerPublicKey = n.Description.TlsInfo.CertIssuerPublicKey
			node.Description.TLSInfo.CertIssuerSubject = n.Description.TlsInfo.CertIssuerSubject
		}
		for _, csi := range n.Description.CsiInfo {
			if csi != nil {
				convertedInfo := types.NodeCSIInfo{
					PluginName:        csi.PluginName,
					NodeID:            csi.NodeId,
					MaxVolumesPerNode: csi.MaxVolumesPerNode,
				}

				if csi.AccessibleTopology != nil {
					convertedInfo.AccessibleTopology = &types.Topology{
						Segments: csi.AccessibleTopology.Segments,
					}
				}

				node.Description.CSIInfo = append(
					node.Description.CSIInfo, convertedInfo,
				)
			}
		}
	}

	// Manager
	if n.ManagerStatus != nil {
		node.ManagerStatus = &types.ManagerStatus{
			Leader:       n.ManagerStatus.Leader,
			Reachability: types.Reachability(strings.ToLower(n.ManagerStatus.Reachability.String())),
			Addr:         n.ManagerStatus.Addr,
		}
	}

	return node
}

// NodeSpecToGRPC converts a NodeSpec to a grpc NodeSpec.
func NodeSpecToGRPC(s types.NodeSpec) (*swarmapi.NodeSpec, error) {
	spec := &swarmapi.NodeSpec{
		Annotations: &swarmapi.Annotations{
			Name:   s.Name,
			Labels: s.Labels,
		},
	}
	if role, ok := swarmapi.NodeRole_value[strings.ToUpper(string(s.Role))]; ok {
		spec.DesiredRole = swarmapi.NodeRole(role)
	} else {
		return nil, fmt.Errorf("invalid Role: %q", s.Role)
	}

	if availability, ok := swarmapi.NodeSpec_Availability_value[strings.ToUpper(string(s.Availability))]; ok {
		spec.Availability = swarmapi.NodeSpec_Availability(availability)
	} else {
		return nil, fmt.Errorf("invalid Availability: %q", s.Availability)
	}

	return spec, nil
}
