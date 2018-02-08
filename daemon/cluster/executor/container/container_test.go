package container // import "github.com/docker/docker/daemon/cluster/executor/container"

import (
	"testing"

	container "github.com/docker/docker/api/types/container"
	swarmapi "github.com/docker/swarmkit/api"
	"github.com/stretchr/testify/require"
)

func TestIsolationConversion(t *testing.T) {
	cases := []struct {
		name string
		from swarmapi.ContainerSpec_Isolation
		to   container.Isolation
	}{
		{name: "default", from: swarmapi.ContainerIsolationDefault, to: container.IsolationDefault},
		{name: "process", from: swarmapi.ContainerIsolationProcess, to: container.IsolationProcess},
		{name: "hyperv", from: swarmapi.ContainerIsolationHyperV, to: container.IsolationHyperV},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			task := swarmapi.Task{
				Spec: swarmapi.TaskSpec{
					Runtime: &swarmapi.TaskSpec_Container{
						Container: &swarmapi.ContainerSpec{
							Image:     "alpine:latest",
							Isolation: c.from,
						},
					},
				},
			}
			config := containerConfig{task: &task}
			require.Equal(t, c.to, config.hostConfig().Isolation)
		})
	}
}
