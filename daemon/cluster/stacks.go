package cluster

// stacks.go provides methods on the Cluster object to implement the StackStore
// interface. These methods wrap around the corresponding methods in
// github.com/docker/stacks/pkg/store, and perform the locking etc. needed.

import (
	"context"

	"github.com/docker/stacks/pkg/interfaces"
	"github.com/docker/stacks/pkg/store"
	"github.com/docker/stacks/pkg/types"
)

func (c *Cluster) AddStack(st types.Stack, sst interfaces.SwarmStack) (string, error) {
	var id string
	// using lockedManagerAction a lot because it elides the need to handle
	// most swarm-specific errors
	err := c.lockedManagerAction(func(ctx context.Context, ns nodeState) error {
		var innerErr error
		id, innerErr = store.AddStack(ctx, ns.controlClient, st, sst)
		return innerErr
	})
	return id, err
}

func (c *Cluster) UpdateStack(id string, st types.StackSpec, sst interfaces.SwarmStackSpec, version uint64) error {
	return c.lockedManagerAction(func(ctx context.Context, ns nodeState) error {
		return store.UpdateStack(ctx, ns.controlClient, id, st, sst, version)
	})
}

func (c *Cluster) DeleteStack(id string) error {
	return c.lockedManagerAction(func(ctx context.Context, ns nodeState) error {
		return store.DeleteStack(ctx, ns.controlClient, id)
	})
}

func (c *Cluster) GetStack(id string) (types.Stack, error) {
	var stack types.Stack
	err := c.lockedManagerAction(func(ctx context.Context, ns nodeState) error {
		var innerErr error
		stack, innerErr = store.GetStack(ctx, ns.controlClient, id)
		return innerErr
	})
	return stack, err
}

func (c *Cluster) GetSwarmStack(id string) (interfaces.SwarmStack, error) {
	var stack interfaces.SwarmStack
	err := c.lockedManagerAction(func(ctx context.Context, ns nodeState) error {
		var innerErr error
		stack, innerErr = store.GetSwarmStack(ctx, ns.controlClient, id)
		return innerErr
	})
	return stack, err
}

func (c *Cluster) ListStacks() ([]types.Stack, error) {
	var stacks []types.Stack
	err := c.lockedManagerAction(func(ctx context.Context, ns nodeState) error {
		var innerErr error
		stacks, innerErr = store.ListStacks(ctx, ns.controlClient)
		return innerErr
	})
	return stacks, err
}

func (c *Cluster) ListSwarmStacks() ([]interfaces.SwarmStack, error) {
	var stacks []interfaces.SwarmStack
	err := c.lockedManagerAction(func(ctx context.Context, ns nodeState) error {
		var innerErr error
		stacks, innerErr = store.ListSwarmStacks(ctx, ns.controlClient)
		return innerErr
	})
	return stacks, err
}
