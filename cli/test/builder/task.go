package builder

import (
	"github.com/docker/docker/api/types/swarm"
	"time"
)

// ATask creates a task builder with default values for a swarm Task.
// Use the Build method to get the built task.
func ATask(taskID string) *TaskBuilder {
	t1 := time.Date(2009, time.November, 10, 23, 0, 0, 0, time.UTC)
	t2 := t1.Add(1 * time.Hour)
	return &TaskBuilder{
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

// TaskBuilder holds a task to be built
type TaskBuilder struct {
	task swarm.Task
}

// ServiceID sets the task service's ID
func (b *TaskBuilder) ServiceID(id string) *TaskBuilder {
	b.task.ServiceID = id
	return b
}

// StatusTimestamp sets the task status timestamp
func (b *TaskBuilder) StatusTimestamp(t time.Time) *TaskBuilder {
	b.task.Status.Timestamp = t
	return b
}

// StatusErr sets the tasks status error
func (b *TaskBuilder) StatusErr(err string) *TaskBuilder {
	b.task.Status.Err = err
	return b
}

// StatusPortStatus sets the tasks port config status
func (b *TaskBuilder) StatusPortStatus(portConfigs []swarm.PortConfig) *TaskBuilder {
	b.task.Status.PortStatus.Ports = portConfigs
	return b
}

// Build returns the built task
func (b *TaskBuilder) Build() swarm.Task {
	return b.task
}
