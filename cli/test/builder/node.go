package builder

import (
	"time"

	"github.com/docker/docker/api/types/swarm"
)

// ANode creates a node builder with default values for a swarm Node.
// Use the Build method to get the built node.
func ANode(id string) *NodeBuilder {
	t1 := time.Date(2009, time.November, 10, 23, 0, 0, 0, time.UTC)
	return &NodeBuilder{
		node: swarm.Node{
			ID: id,
			Meta: swarm.Meta{
				CreatedAt: t1,
			},
			Description: swarm.NodeDescription{
				Hostname: "defaultNodeHostname",
				Platform: swarm.Platform{
					Architecture: "x86_64",
					OS:           "linux",
				},
				Resources: swarm.Resources{
					NanoCPUs:    4,
					MemoryBytes: 20 * 1024 * 1024,
				},
				Engine: swarm.EngineDescription{
					EngineVersion: "1.13.0",
					Labels: map[string]string{
						"engine": "label",
					},
					Plugins: []swarm.PluginDescription{
						{
							Type: "Volume",
							Name: "local",
						},
						{
							Type: "Network",
							Name: "bridge",
						},
						{
							Type: "Network",
							Name: "overlay",
						},
					},
				},
			},
			Status: swarm.NodeStatus{
				State: swarm.NodeStateReady,
				Addr:  "127.0.0.1",
			},
			Spec: swarm.NodeSpec{
				Annotations: swarm.Annotations{
					Name: "defaultNodeName",
				},
				Role:         swarm.NodeRoleWorker,
				Availability: swarm.NodeAvailabilityActive,
			},
		},
	}
}

// NodeBuilder holds a node to be built
type NodeBuilder struct {
	node swarm.Node
}

// Build returns the built node
func (b *NodeBuilder) Build() swarm.Node {
	return b.node
}

// Labels sets the node labels
func (b *NodeBuilder) Labels(labels map[string]string) *NodeBuilder {
	b.node.Spec.Labels = labels
	return b
}

// Hostname sets the node hostname
func (b *NodeBuilder) Hostname(hostname string) *NodeBuilder {
	b.node.Description.Hostname = hostname
	return b
}

// Leader sets the current node as a leader
func (b *NodeBuilder) Leader() *NodeBuilder {
	b.node.ManagerStatus.Leader = true
	return b
}

// Manager set the current node as a manager
func (b *NodeBuilder) Manager() *NodeBuilder {
	b.node.Spec.Role = swarm.NodeRoleManager
	b.node.ManagerStatus = &swarm.ManagerStatus{
		Reachability: swarm.ReachabilityReachable,
		Addr:         "127.0.0.1",
	}
	return b
}
