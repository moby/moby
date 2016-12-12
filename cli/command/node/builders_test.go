package node

import (
	"time"

	"github.com/docker/docker/api/types/swarm"
)

func aNode(id string) *nodeBuilder {
	t1 := time.Date(2009, time.November, 10, 23, 0, 0, 0, time.UTC)
	return &nodeBuilder{
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

type nodeBuilder struct {
	node swarm.Node
}

func (b *nodeBuilder) build() swarm.Node {
	return b.node
}

func (b *nodeBuilder) labels(labels map[string]string) *nodeBuilder {
	b.node.Spec.Labels = labels
	return b
}

func (b *nodeBuilder) hostname(hostname string) *nodeBuilder {
	b.node.Description.Hostname = hostname
	return b
}

func (b *nodeBuilder) leader() *nodeBuilder {
	b.node.ManagerStatus.Leader = true
	return b
}

func (b *nodeBuilder) manager() *nodeBuilder {
	b.node.Spec.Role = swarm.NodeRoleManager
	b.node.ManagerStatus = &swarm.ManagerStatus{
		Reachability: swarm.ReachabilityReachable,
		Addr:         "127.0.0.1",
	}
	return b
}

func aTask(taskID string) *taskBuilder {
	t1 := time.Date(2009, time.November, 10, 23, 0, 0, 0, time.UTC)
	t2 := t1.Add(1 * time.Hour)
	return &taskBuilder{
		task: swarm.Task{
			ID: taskID,
			Meta: swarm.Meta{
				CreatedAt: t1,
			},
			Annotations: swarm.Annotations{
				Name: "defaultTaskName",
			},
			Spec: swarm.TaskSpec{
				ContainerSpec: swarm.ContainerSpec{
					Image: "myimage:mytag",
				},
			},
			ServiceID: "rl02d5gwz6chzu7il5fhtb8be",
			Slot:      1,
			Status: swarm.TaskStatus{
				State:     swarm.TaskStateReady,
				Timestamp: t2,
			},
			DesiredState: swarm.TaskStateReady,
		},
	}
}

type taskBuilder struct {
	task swarm.Task
}

func (b *taskBuilder) serviceID(id string) *taskBuilder {
	b.task.ServiceID = id
	return b
}

func (b *taskBuilder) statusTimeStamp(t time.Time) *taskBuilder {
	b.task.Status.Timestamp = t
	return b
}

func (b *taskBuilder) statusErr(err string) *taskBuilder {
	b.task.Status.Err = err
	return b
}

func (b *taskBuilder) statusPortStatus(portConfigs []swarm.PortConfig) *taskBuilder {
	b.task.Status.PortStatus.Ports = portConfigs
	return b
}

func (b *taskBuilder) build() swarm.Task {
	return b.task
}
