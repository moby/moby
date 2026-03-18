package network

import (
	"context"
	"testing"

	"github.com/moby/moby/client"
	"gotest.tools/v3/assert"
)

func createNetwork(ctx context.Context, apiClient client.APIClient, name string, ops ...func(*client.NetworkCreateOptions)) (string, error) {
	config := client.NetworkCreateOptions{}

	for _, op := range ops {
		op(&config)
	}

	n, err := apiClient.NetworkCreate(ctx, name, config)
	return n.ID, err
}

// Create creates a network with the specified options
func Create(ctx context.Context, apiClient client.APIClient, name string, ops ...func(*client.NetworkCreateOptions)) (string, error) {
	return createNetwork(ctx, apiClient, name, ops...)
}

// CreateNoError creates a network with the specified options and verifies there were no errors
func CreateNoError(ctx context.Context, t *testing.T, apiClient client.APIClient, name string, ops ...func(*client.NetworkCreateOptions)) string {
	t.Helper()

	name, err := createNetwork(ctx, apiClient, name, ops...)
	assert.NilError(t, err)
	return name
}

// Inspect inspects a network with the specified options
func Inspect(ctx context.Context, apiClient client.APIClient, name string, options client.NetworkInspectOptions) (client.NetworkInspectResult, error) {
	return apiClient.NetworkInspect(ctx, name, options)
}

// InspectNoError inspects a network with the specified options and verifies there were no errors
func InspectNoError(ctx context.Context, t *testing.T, apiClient client.APIClient, name string, options client.NetworkInspectOptions) client.NetworkInspectResult {
	t.Helper()

	c, err := apiClient.NetworkInspect(ctx, name, options)
	assert.NilError(t, err)

	return c
}

func RemoveNoError(ctx context.Context, t *testing.T, apiClient client.APIClient, name string) {
	t.Helper()

	_, err := apiClient.NetworkRemove(ctx, name, client.NetworkRemoveOptions{})
	assert.NilError(t, err)
}
