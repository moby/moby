package builders

import (
	"time"

	"github.com/docker/docker/api/types/swarm"
)

// Node creates a node with default values.
// Any number of node function builder can be pass to augment it.
//
//	n1 := Node() // Returns a default node
//	n2 := Node(NodeID("foo"), NodeHostname("bar"), Leader())
func Node(builders ...func(*swarm.Node)) *swarm.Node {
	t1 := time.Date(2009, time.November, 10, 23, 0, 0, 0, time.UTC)
	node := &swarm.Node{
		ID: "nodeID",
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
	}

	for _, builder := range builders {
		builder(node)
	}

	return node
}

// NodeID sets the node id
func NodeID(id string) func(*swarm.Node) {
	return func(node *swarm.Node) {
		node.ID = id
	}
}

// NodeName sets the node name
func NodeName(name string) func(*swarm.Node) {
	return func(node *swarm.Node) {
		node.Spec.Annotations.Name = name
	}
}

// NodeLabels sets the node labels
func NodeLabels(labels map[string]string) func(*swarm.Node) {
	return func(node *swarm.Node) {
		node.Spec.Labels = labels
	}
}

// Hostname sets the node hostname
func Hostname(hostname string) func(*swarm.Node) {
	return func(node *swarm.Node) {
		node.Description.Hostname = hostname
	}
}

// Leader sets the current node as a leader
func Leader() func(*swarm.ManagerStatus) {
	return func(managerStatus *swarm.ManagerStatus) {
		managerStatus.Leader = true
	}
}

// Manager set the current node as a manager
func Manager(managerStatusBuilders ...func(*swarm.ManagerStatus)) func(*swarm.Node) {
	return func(node *swarm.Node) {
		node.Spec.Role = swarm.NodeRoleManager
		node.ManagerStatus = ManagerStatus(managerStatusBuilders...)
	}
}

// ManagerStatus create a ManageStatus with default values.
func ManagerStatus(managerStatusBuilders ...func(*swarm.ManagerStatus)) *swarm.ManagerStatus {
	managerStatus := &swarm.ManagerStatus{
		Reachability: swarm.ReachabilityReachable,
		Addr:         "127.0.0.1",
	}

	for _, builder := range managerStatusBuilders {
		builder(managerStatus)
	}

	return managerStatus
}
