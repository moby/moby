package container

import (
	"testing"

	"github.com/docker/swarmkit/api"
	"github.com/stretchr/testify/assert"
)

func TestContainerLabels(t *testing.T) {
	c := &containerConfig{
		task: &api.Task{
			ID: "real-task.id",
			Spec: api.TaskSpec{
				Runtime: &api.TaskSpec_Container{
					Container: &api.ContainerSpec{
						Labels: map[string]string{
							"com.docker.swarm.task":         "user-specified-task",
							"com.docker.swarm.task.id":      "user-specified-task.id",
							"com.docker.swarm.task.name":    "user-specified-task.name",
							"com.docker.swarm.task.slot":    "user-specified-task.slot",
							"com.docker.swarm.node.id":      "user-specified-node.id",
							"com.docker.swarm.service.id":   "user-specified-service.id",
							"com.docker.swarm.service.name": "user-specified-service.name",
							"this-is-a-user-label":          "this is a user label's value",
						},
					},
				},
			},
			ServiceID: "real-service.id",
			Slot:      123,
			NodeID:    "real-node.id",
			Annotations: api.Annotations{
				Name: "real-service.name.123.real-task.id",
			},
			ServiceAnnotations: api.Annotations{
				Name: "real-service.name",
			},
		},
	}

	expected := map[string]string{
		"com.docker.swarm.task":         "",
		"com.docker.swarm.task.id":      "real-task.id",
		"com.docker.swarm.task.name":    "real-service.name.123.real-task.id",
		"com.docker.swarm.task.slot":    "123",
		"com.docker.swarm.node.id":      "real-node.id",
		"com.docker.swarm.service.id":   "real-service.id",
		"com.docker.swarm.service.name": "real-service.name",
		"this-is-a-user-label":          "this is a user label's value",
	}

	labels := c.labels()
	assert.Len(t, labels, 8)
	assert.Equal(t, expected, labels)
}

func TestContainerLabelsGlobalService(t *testing.T) {
	c := &containerConfig{
		task: &api.Task{
			ID: "real-task.id",
			Spec: api.TaskSpec{
				Runtime: &api.TaskSpec_Container{
					Container: &api.ContainerSpec{
						Labels: map[string]string{
							"com.docker.swarm.task":         "user-specified-task",
							"com.docker.swarm.task.id":      "user-specified-task.id",
							"com.docker.swarm.task.name":    "user-specified-task.name",
							"com.docker.swarm.task.slot":    "user-specified-task.slot",
							"com.docker.swarm.node.id":      "user-specified-node.id",
							"com.docker.swarm.service.id":   "user-specified-service.id",
							"com.docker.swarm.service.name": "user-specified-service.name",
							"this-is-a-user-label":          "this is a user label's value",
						},
					},
				},
			},
			ServiceID: "real-service.id",
			Slot:      0,
			NodeID:    "real-node.id",
			Annotations: api.Annotations{
				Name: "real-service.name.real-task.id",
			},
			ServiceAnnotations: api.Annotations{
				Name: "real-service.name",
			},
		},
	}

	expected := map[string]string{
		"com.docker.swarm.task":         "",
		"com.docker.swarm.task.id":      "real-task.id",
		"com.docker.swarm.task.name":    "real-service.name.real-task.id",
		"com.docker.swarm.task.slot":    "user-specified-task.slot",
		"com.docker.swarm.node.id":      "real-node.id",
		"com.docker.swarm.service.id":   "real-service.id",
		"com.docker.swarm.service.name": "real-service.name",
		"this-is-a-user-label":          "this is a user label's value",
	}

	labels := c.labels()
	assert.Len(t, labels, 8)
	assert.Equal(t, expected, labels)
}
