package client

import (
	"context"
	"encoding/json"

	"github.com/moby/moby/api/types/swarm"
)

// TaskInspectOptions contains options for inspecting a task.
type TaskInspectOptions struct {
	// Currently no options are defined.
}

// TaskInspectResult contains the result of a task inspection.
type TaskInspectResult struct {
	Task swarm.Task
	Raw  json.RawMessage
}

// TaskInspect returns the task information and its raw representation.
func (cli *Client) TaskInspect(ctx context.Context, taskID string, options TaskInspectOptions) (TaskInspectResult, error) {
	taskID, err := trimID("task", taskID)
	if err != nil {
		return TaskInspectResult{}, err
	}

	resp, err := cli.get(ctx, "/tasks/"+taskID, nil, nil)
	if err != nil {
		return TaskInspectResult{}, err
	}

	var out TaskInspectResult
	out.Raw, err = decodeWithRaw(resp, &out.Task)
	return out, err
}
