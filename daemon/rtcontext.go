package daemon

import (
	"path/filepath"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/container"
)

type runtimeTaskContext struct {
	ID   string
	Name string
	Slot int
}

type runtimeServiceContext struct {
	ID   string
	Name string
}

// RuntimeContext is introspection data for a container
type RuntimeContext struct {
	Container struct {
		ID string
		// Name does not have '/' prefix but FullName has
		Name     string
		FullName string
		Labels   map[string]string
	}
	Daemon struct {
		Name   string
		Labels map[string]string
	}
	// nil for non-task container
	Task *runtimeTaskContext
	// nil for non-task container
	Service *runtimeServiceContext
}

// introspectRuntimeContext returns RuntimeContext.
// Error is printed to logrus, and unknown field is set to an empty value.
// TODO: support dynamic label update
func (daemon *Daemon) introspectRuntimeContext(c *container.Container) RuntimeContext {
	ctx := RuntimeContext{}
	ctx.Container.ID = c.ID
	ctx.Container.Name = filepath.Base(c.Name)
	ctx.Container.FullName = c.Name
	ctx.Container.Labels = c.Config.Labels

	if info, err := daemon.SystemInfo(); err != nil {
		logrus.Warnf("error while introspecting daemon: %v", err)
	} else {
		ctx.Daemon.Name = info.Name
		m := make(map[string]string, 0)
		for _, s := range info.Labels {
			m[s] = ""
		}
		ctx.Daemon.Labels = m
	}
	if cluster := daemon.GetCluster(); cluster != nil {
		ctx.Task = introspectRuntimeTaskContext(c, cluster)
		ctx.Service = introspectRuntimeServiceContext(c, cluster)
	}
	return ctx
}

func introspectRuntimeTaskContext(c *container.Container, cluster Cluster) *runtimeTaskContext {
	taskID, ok := c.Config.Labels["com.docker.swarm.task.id"]
	if !ok {
		return nil
	}
	task, err := cluster.GetTask(taskID)
	if err != nil {
		logrus.Warnf("error while introspecting task %s: %v",
			taskID, err)
		return nil
	}
	taskName, ok := c.Config.Labels["com.docker.swarm.task.name"]
	if !ok {
		taskName = ""
	}
	return &runtimeTaskContext{
		ID:   task.ID,
		Name: taskName,
		Slot: task.Slot,
	}
}

func introspectRuntimeServiceContext(c *container.Container, cluster Cluster) *runtimeServiceContext {
	serviceID, ok := c.Config.Labels["com.docker.swarm.service.id"]
	if !ok {
		return nil
	}
	service, err := cluster.GetService(serviceID, true)
	if err != nil {
		logrus.Warnf("error while introspecting service %s: %v",
			serviceID, err)
		return nil
	}
	return &runtimeServiceContext{
		ID:   service.ID,
		Name: service.Spec.Name,
	}
}
