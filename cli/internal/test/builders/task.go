package builders

import (
	"time"

	"github.com/docker/docker/api/types/swarm"
)

var (
	defaultTime = time.Date(2009, time.November, 10, 23, 0, 0, 0, time.UTC)
)

// Task creates a task with default values .
// Any number of task function builder can be pass to augment it.
func Task(taskBuilders ...func(*swarm.Task)) *swarm.Task {
	task := &swarm.Task{
		ID: "taskID",
		Meta: swarm.Meta{
			CreatedAt: defaultTime,
		},
		Annotations: swarm.Annotations{
			Name: "defaultTaskName",
		},
		Spec:         *TaskSpec(),
		ServiceID:    "rl02d5gwz6chzu7il5fhtb8be",
		Slot:         1,
		Status:       *TaskStatus(),
		DesiredState: swarm.TaskStateReady,
	}

	for _, builder := range taskBuilders {
		builder(task)
	}

	return task
}

// TaskID sets the task ID
func TaskID(id string) func(*swarm.Task) {
	return func(task *swarm.Task) {
		task.ID = id
	}
}

// ServiceID sets the task service's ID
func ServiceID(id string) func(*swarm.Task) {
	return func(task *swarm.Task) {
		task.ServiceID = id
	}
}

// WithStatus sets the task status
func WithStatus(statusBuilders ...func(*swarm.TaskStatus)) func(*swarm.Task) {
	return func(task *swarm.Task) {
		task.Status = *TaskStatus(statusBuilders...)
	}
}

// TaskStatus creates a task status with default values .
// Any number of taskStatus function builder can be pass to augment it.
func TaskStatus(statusBuilders ...func(*swarm.TaskStatus)) *swarm.TaskStatus {
	timestamp := defaultTime.Add(1 * time.Hour)
	taskStatus := &swarm.TaskStatus{
		State:     swarm.TaskStateReady,
		Timestamp: timestamp,
	}

	for _, builder := range statusBuilders {
		builder(taskStatus)
	}

	return taskStatus
}

// Timestamp sets the task status timestamp
func Timestamp(t time.Time) func(*swarm.TaskStatus) {
	return func(taskStatus *swarm.TaskStatus) {
		taskStatus.Timestamp = t
	}
}

// StatusErr sets the tasks status error
func StatusErr(err string) func(*swarm.TaskStatus) {
	return func(taskStatus *swarm.TaskStatus) {
		taskStatus.Err = err
	}
}

// PortStatus sets the tasks port config status
// FIXME(vdemeester) should be a sub builder 👼
func PortStatus(portConfigs []swarm.PortConfig) func(*swarm.TaskStatus) {
	return func(taskStatus *swarm.TaskStatus) {
		taskStatus.PortStatus.Ports = portConfigs
	}
}

// TaskSpec creates a task spec with default values .
// Any number of taskSpec function builder can be pass to augment it.
func TaskSpec(specBuilders ...func(*swarm.TaskSpec)) *swarm.TaskSpec {
	taskSpec := &swarm.TaskSpec{
		ContainerSpec: swarm.ContainerSpec{
			Image: "myimage:mytag",
		},
	}

	for _, builder := range specBuilders {
		builder(taskSpec)
	}

	return taskSpec
}
