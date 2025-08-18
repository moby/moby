package network

import (
	"context"
	"testing"

	"gotest.tools/v3/assert"

	"github.com/moby/moby/api/types/network"
	"github.com/moby/moby/client"
)

func createNetwork(ctx context.Context, client client.APIClient, name string, ops ...func(*network.CreateOptions)) (string, error) {
	config := network.CreateOptions{}

	for _, op := range ops {
		op(&config)
	}

	n, err := client.NetworkCreate(ctx, name, config)
	return n.ID, err
}

// Create creates a network with the specified options
func Create(ctx context.Context, client client.APIClient, name string, ops ...func(*network.CreateOptions)) (string, error) {
	return createNetwork(ctx, client, name, ops...)
}

// CreateNoError creates a network with the specified options and verifies there were no errors
func CreateNoError(ctx context.Context, t *testing.T, client client.APIClient, name string, ops ...func(*network.CreateOptions)) string {
	t.Helper()

	name, err := createNetwork(ctx, client, name, ops...)
	assert.NilError(t, err)
	return name
}

func RemoveNoError(ctx context.Context, t *testing.T, apiClient client.APIClient, name string) {
	t.Helper()

	err := apiClient.NetworkRemove(ctx, name)
	assert.NilError(t, err)
}
