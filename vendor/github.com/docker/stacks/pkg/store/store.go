package store

import (
	"context"

	"github.com/docker/stacks/pkg/interfaces"
	"github.com/docker/stacks/pkg/types"
)

// StackStore is an implementation of the interfaces.StackStore interface,
// which provides for the storage and retrieval of Stack objects from the
// swarmkit object store.
type StackStore struct {
	client ResourcesClient
}

// New creates a new StackStore using the provided client.
func New(client ResourcesClient) *StackStore {
	return &StackStore{
		client: client,
	}
}

// AddStack creates a new Stack object in the swarmkit data store. It returns
// the ID of the new object if successful, or an error otherwise.
func (s *StackStore) AddStack(st types.Stack, sst interfaces.SwarmStack) (string, error) {
	return AddStack(context.TODO(), s.client, st, sst)
}

// UpdateStack updates an existing Stack object
func (s *StackStore) UpdateStack(id string, st types.StackSpec, sst interfaces.SwarmStackSpec, version uint64) error {
	return UpdateStack(context.TODO(), s.client, id, st, sst, version)
}

// DeleteStack removes the stacks with the given ID.
func (s *StackStore) DeleteStack(id string) error {
	return DeleteStack(context.TODO(), s.client, id)
}

// GetStack retrieves and returns an existing types.Stack object by ID
func (s *StackStore) GetStack(id string) (types.Stack, error) {
	return GetStack(context.TODO(), s.client, id)
}

// GetSwarmStack retrieves and returns an exist types.SwarmStack object by ID.
func (s *StackStore) GetSwarmStack(id string) (interfaces.SwarmStack, error) {
	return GetSwarmStack(context.TODO(), s.client, id)
}

// ListStacks lists all available stack objects
func (s *StackStore) ListStacks() ([]types.Stack, error) {
	return ListStacks(context.TODO(), s.client)
}

// ListSwarmStacks lists all available stack objects as SwarmStacks
func (s *StackStore) ListSwarmStacks() ([]interfaces.SwarmStack, error) {
	return ListSwarmStacks(context.TODO(), s.client)
}
