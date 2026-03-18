package daemon

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/moby/moby/api/types/swarm"
	"github.com/moby/moby/client"
	"gotest.tools/v3/assert"
)

// CheckServiceTasksInState returns the number of tasks with a matching state,
// and optional message substring.
func (d *Daemon) CheckServiceTasksInState(ctx context.Context, service string, state swarm.TaskState, message string) func(*testing.T) (any, string) {
	return func(t *testing.T) (any, string) {
		tasks := d.GetServiceTasks(ctx, t, service)
		var count int
		for _, task := range tasks {
			if task.Status.State == state {
				if message == "" || strings.Contains(task.Status.Message, message) {
					count++
				}
			}
		}
		return count, ""
	}
}

// CheckServiceTasksInStateWithError returns the number of tasks with a matching state,
// and optional message substring.
func (d *Daemon) CheckServiceTasksInStateWithError(ctx context.Context, service string, state swarm.TaskState, errorMessage string) func(*testing.T) (any, string) {
	return func(t *testing.T) (any, string) {
		tasks := d.GetServiceTasks(ctx, t, service)
		var count int
		for _, task := range tasks {
			if task.Status.State == state {
				if errorMessage == "" || strings.Contains(task.Status.Err, errorMessage) {
					count++
				}
			}
		}
		return count, ""
	}
}

// CheckServiceRunningTasks returns the number of running tasks for the specified service
func (d *Daemon) CheckServiceRunningTasks(ctx context.Context, service string) func(*testing.T) (any, string) {
	return d.CheckServiceTasksInState(ctx, service, swarm.TaskStateRunning, "")
}

// CheckServiceUpdateState returns the current update state for the specified service
func (d *Daemon) CheckServiceUpdateState(ctx context.Context, service string) func(*testing.T) (any, string) {
	return func(t *testing.T) (any, string) {
		service := d.GetService(ctx, t, service)
		if service.UpdateStatus == nil {
			return "", ""
		}
		return service.UpdateStatus.State, ""
	}
}

// CheckPluginRunning returns the runtime state of the plugin
func (d *Daemon) CheckPluginRunning(ctx context.Context, plugin string) func(c *testing.T) (any, string) {
	return func(t *testing.T) (any, string) {
		apiclient := d.NewClientT(t)
		resp, err := apiclient.PluginInspect(ctx, plugin, client.PluginInspectOptions{})
		if cerrdefs.IsNotFound(err) {
			return false, fmt.Sprintf("%v", err)
		}
		assert.NilError(t, err)
		return resp.Plugin.Enabled, fmt.Sprintf("%+v", resp.Plugin)
	}
}

// CheckPluginImage returns the runtime state of the plugin
func (d *Daemon) CheckPluginImage(ctx context.Context, plugin string) func(c *testing.T) (any, string) {
	return func(t *testing.T) (any, string) {
		apiclient := d.NewClientT(t)
		resp, err := apiclient.PluginInspect(ctx, plugin, client.PluginInspectOptions{})
		if cerrdefs.IsNotFound(err) {
			return false, fmt.Sprintf("%v", err)
		}
		assert.NilError(t, err)
		return resp.Plugin.PluginReference, fmt.Sprintf("%+v", resp.Plugin)
	}
}

// CheckServiceTasks returns the number of tasks for the specified service
func (d *Daemon) CheckServiceTasks(ctx context.Context, service string) func(*testing.T) (any, string) {
	return func(t *testing.T) (any, string) {
		tasks := d.GetServiceTasks(ctx, t, service)
		return len(tasks), ""
	}
}

// CheckRunningTaskNetworks returns the number of times each network is referenced from a task.
func (d *Daemon) CheckRunningTaskNetworks(ctx context.Context) func(t *testing.T) (any, string) {
	return func(t *testing.T) (any, string) {
		cli := d.NewClientT(t)
		defer cli.Close()

		taskList, err := cli.TaskList(ctx, client.TaskListOptions{
			Filters: make(client.Filters).Add("desired-state", "running"),
		})
		assert.NilError(t, err)

		result := make(map[string]int)
		for _, task := range taskList.Items {
			for _, network := range task.Spec.Networks {
				result[network.Target]++
			}
		}
		return result, ""
	}
}

// CheckRunningTaskImages returns the times each image is running as a task.
func (d *Daemon) CheckRunningTaskImages(ctx context.Context) func(t *testing.T) (any, string) {
	return func(t *testing.T) (any, string) {
		cli := d.NewClientT(t)
		defer cli.Close()

		taskList, err := cli.TaskList(ctx, client.TaskListOptions{
			Filters: make(client.Filters).Add("desired-state", "running"),
		})
		assert.NilError(t, err)

		result := make(map[string]int)
		for _, task := range taskList.Items {
			if task.Status.State == swarm.TaskStateRunning && task.Spec.ContainerSpec != nil {
				result[task.Spec.ContainerSpec.Image]++
			}
		}
		return result, ""
	}
}

// CheckNodeReadyCount returns the number of ready node on the swarm
func (d *Daemon) CheckNodeReadyCount(ctx context.Context) func(t *testing.T) (any, string) {
	return func(t *testing.T) (any, string) {
		nodes := d.ListNodes(ctx, t)
		var readyCount int
		for _, node := range nodes {
			if node.Status.State == swarm.NodeStateReady {
				readyCount++
			}
		}
		return readyCount, ""
	}
}

// CheckLocalNodeState returns the current swarm node state
func (d *Daemon) CheckLocalNodeState(ctx context.Context) func(t *testing.T) (any, string) {
	return func(t *testing.T) (any, string) {
		info := d.SwarmInfo(ctx, t)
		return info.LocalNodeState, ""
	}
}

// CheckControlAvailable returns the current swarm control available
func (d *Daemon) CheckControlAvailable(ctx context.Context) func(t *testing.T) (any, string) {
	return func(t *testing.T) (any, string) {
		info := d.SwarmInfo(ctx, t)
		assert.Equal(t, info.LocalNodeState, swarm.LocalNodeStateActive)
		return info.ControlAvailable, ""
	}
}

// CheckLeader returns whether there is a leader on the swarm or not
func (d *Daemon) CheckLeader(ctx context.Context) func(t *testing.T) (any, string) {
	return func(t *testing.T) (any, string) {
		cli := d.NewClientT(t)
		defer cli.Close()

		errList := "could not get node list"

		result, err := cli.NodeList(ctx, client.NodeListOptions{})
		if err != nil {
			return err, errList
		}

		for _, node := range result.Items {
			if node.ManagerStatus != nil && node.ManagerStatus.Leader {
				return nil, ""
			}
		}
		return errors.New("no leader"), "could not find leader"
	}
}

// CmdRetryOutOfSequence tries the specified command against the current daemon
// up to 10 times, retrying if it encounters an "update out of sequence" error.
func (d *Daemon) CmdRetryOutOfSequence(args ...string) (string, error) {
	var (
		output string
		err    error
	)

	for range 10 {
		output, err = d.Cmd(args...)
		// error, no error, whatever. if we don't have "update out of
		// sequence", we don't retry, we just return.
		if !strings.Contains(output, "update out of sequence") {
			return output, err
		}
	}

	// otherwise, once all of our attempts have been exhausted, just return
	// whatever the last values were.
	return output, err
}
