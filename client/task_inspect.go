package client

import (
	"context"

	"github.com/moby/moby/api/types/swarm"
)

// TaskInspectResult contains the result of a task inspection.
type TaskInspectResult struct {
	Task swarm.Task
	Raw  []byte
}

// TaskInspect returns the task information and its raw representation.
func (cli *Client) TaskInspect(ctx context.Context, taskID string) (TaskInspectResult, error) {
	taskID, err := trimID("task", taskID)
	if err != nil {
		return TaskInspectResult{}, err
	}

	resp, err := cli.get(ctx, "/tasks/"+taskID, nil, nil)
	defer ensureReaderClosed(resp)
	if err != nil {
		return TaskInspectResult{}, err
	}

	var out TaskInspectResult
	out.Raw, err = decodeWithRaw(resp, &out.Task)
	return out, err
}
