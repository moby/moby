package client

import (
	"context"
	"encoding/json"
	"net/url"

	"github.com/moby/moby/api/types/swarm"
)

// TaskListOptions holds parameters to list tasks with.
type TaskListOptions struct {
	Filters Filters
}

// TaskListResult contains the result of a task list operation.
type TaskListResult struct {
	Items []swarm.Task
}

// TaskList returns the list of tasks.
func (cli *Client) TaskList(ctx context.Context, options TaskListOptions) (TaskListResult, error) {
	query := url.Values{}

	options.Filters.updateURLValues(query)

	resp, err := cli.get(ctx, "/tasks", query, nil)
	defer ensureReaderClosed(resp)
	if err != nil {
		return TaskListResult{}, err
	}

	var tasks []swarm.Task
	err = json.NewDecoder(resp.Body).Decode(&tasks)
	return TaskListResult{Items: tasks}, err
}
