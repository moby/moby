package client // import "github.com/docker/docker/client"

import (
	"context"
	"encoding/json"
	"net/url"

	"github.com/docker/docker/api/types/stack"
	"github.com/docker/docker/api/types/swarm"
)

// TODO(dperny): in the final implementation, these methods will all exist in
// separate files, like all other client methods.

// StackList returns a list of all stacks
func (cli *Client) StackList(ctx context.Context, options stacks.StackListOptions) ([]stacks.Stack, error) {
	return nil, nil
}

// StackCreate creates a server-side stack
func (cli *Client) StackCreate(ctx context.Context, stack stacks.StackSpec, options types.StackCreateOptions) (stacks.StackCreateResponse, error) {
	return stacks.StackCreateResponse{}, nil
}

// StackInspectWithRaw returns the stack information and the raw data.
//
// A key use of StackInspectWithRaw is for future cross-orchestrator use of
// Stacks. Basically, other orchestrators may need additional fields on the
// stack object that the docker engine itself does not know about. returning
// the raw response allows reuse of this same client with an expanded API
func (cli *Client) StackInspectWithRaw(ctx context.Context, id string, opts types.StackInspectOptions) (stacks.Stack, []byte, error) {
	return stack.Stack{}, []byte{}, nil
}

// StackUpdate updates the stack with the given ID. The version number is
// required to avoid conflicting writes. It should be the value as set before
// the update.
func (cli *Client) StackUpdate(ctx context.Context, id string, version swarm.Version, stack stacks.StackSpec, options types.StackUpdateOptions) (stacks.StackUpdateResponse, error) {
	return stacks.StackUpdateResponse{}, nil
}

// StackRemove removes the stack with the given ID
func (cli *Client) StackRemove(ctx context.Context, id string) error {
	return nil
}
