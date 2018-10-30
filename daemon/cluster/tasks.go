package cluster // import "github.com/docker/docker/daemon/cluster"

import (
	"context"
	"io"

	apitypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	types "github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/daemon/cluster/convert"
	swarmapi "github.com/docker/swarmkit/api"
	"github.com/pkg/errors"
)

// GetTasks returns a list of tasks matching the filter options.
func (c *Cluster) GetTasks(options apitypes.TaskListOptions) ([]types.Task, error) {
	swarmTasks := []*swarmapi.Task{}

	if err := c.lockedManagerAction(func(ctx context.Context, state nodeState) error {
		filterTransform := func(filter filters.Args) error {
			if filter.Contains("service") {
				serviceFilters := filter.Get("service")
				for _, serviceFilter := range serviceFilters {
					service, err := getService(ctx, state.controlClient, serviceFilter, false)
					if err != nil {
						return err
					}
					filter.Del("service", serviceFilter)
					filter.Add("service", service.ID)
				}
			}
			if filter.Contains("node") {
				nodeFilters := filter.Get("node")
				for _, nodeFilter := range nodeFilters {
					node, err := getNode(ctx, state.controlClient, nodeFilter)
					if err != nil {
						return err
					}
					filter.Del("node", nodeFilter)
					filter.Add("node", node.ID)
				}
			}
			if !filter.Contains("runtime") {
				// default to only showing container tasks
				filter.Add("runtime", "container")
				filter.Add("runtime", "")
			}
			return nil
		}

		filters, err := newListTasksFilters(options.Filters, filterTransform)
		if err != nil {
			return err
		}

		stream, err := state.controlClient.ListTasksStream(
			ctx,
			&swarmapi.ListTasksRequest{Filters: filters},
		)
		if err != nil {
			return err
		}

		// iterate through the stream
		for {
			resp, err := stream.Recv()
			// io.EOF signifies that the stream is finished
			if err == io.EOF {
				return nil
			}
			// but any other error signifies a problem
			if err != nil {
				return err
			}
			if resp == nil {
				// this should not be possible
				return errors.New("recieved nil response in stream")
			}

			// otherwise, add all of the tasks in this response to the total of
			// all tasks
			swarmTasks = append(swarmTasks, resp.Tasks...)
		}
		return err
	}); err != nil {
		return nil, err
	}

	tasks := make([]types.Task, 0, len(swarmTasks))
	for _, task := range swarmTasks {
		t, err := convert.TaskFromGRPC(*task)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}
	return tasks, nil
}

// GetTask returns a task by an ID.
func (c *Cluster) GetTask(input string) (types.Task, error) {
	var task *swarmapi.Task
	if err := c.lockedManagerAction(func(ctx context.Context, state nodeState) error {
		t, err := getTask(ctx, state.controlClient, input)
		if err != nil {
			return err
		}
		task = t
		return nil
	}); err != nil {
		return types.Task{}, err
	}
	return convert.TaskFromGRPC(*task)
}
