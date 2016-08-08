package daemon

import (
	"encoding/json"
	"fmt"

	"github.com/docker/docker/container"
	"github.com/docker/engine-api/types"
	"github.com/docker/engine-api/types/swarm"
)

// inspectionFSConnector implements github.com/docker/docker/inspectionfs.DaemonConnector
type inspectionFSConnector struct {
	daemon    *Daemon
	container *container.Container
}

func (c *inspectionFSConnector) ReadJSON(path string) ([]byte, error) {
	marshal := func(v interface{}) ([]byte, error) {
		b, err := json.MarshalIndent(v, "", "    ")
		if err != nil {
			return nil, err
		}
		return append(b, []byte("\n")...), nil
	}
	switch path {
	case "container/json":
		ctx := &listContext{
			names: map[string][]string{c.container.ID: {c.container.Name}},
			ContainerListOptions: &types.ContainerListOptions{
				Size: false,
			},
		}
		container, err := c.daemon.transformContainer(c.container, ctx)
		if err != nil {
			return nil, err
		}
		return marshal(container)
	case "swarm/task/json":
		task, err := c.getTask()
		if err != nil {
			return nil, err
		}
		return marshal(task)
	}
	return nil, fmt.Errorf("unknown inspectionfs json path %s", path)
}

func (c *inspectionFSConnector) getTask() (*swarm.Task, error) {
	taskIDLabel := "com.docker.swarm.task.id"
	taskID, ok := c.container.Config.Labels[taskIDLabel]
	if !ok {
		return nil, fmt.Errorf("container %s seems not a Swarm task", c.container.ID)
	}
	cluster := c.daemon.GetCluster()
	if cluster == nil {
		return nil, fmt.Errorf("Swarm seems not enabled")
	}
	task, err := cluster.GetTask(taskID)
	if err != nil {
		return nil, err
	}
	return &task, nil
}
