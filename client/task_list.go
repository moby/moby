package client

import (
	"context"
	"encoding/json"
	"net/url"

	"github.com/moby/moby/api/types/swarm"
)

// TaskList returns the list of tasks.
func (cli *Client) TaskList(ctx context.Context, options TaskListOptions) ([]swarm.Task, error) {
	query := url.Values{}

	options.Filters.updateURLValues(query)

	resp, err := cli.get(ctx, "/tasks", query, nil)
	defer ensureReaderClosed(resp)
	if err != nil {
		return nil, err
	}

	var tasks []swarm.Task
	err = json.NewDecoder(resp.Body).Decode(&tasks)
	return tasks, err
}
