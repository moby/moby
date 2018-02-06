package container

import (
	"context"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/integration/util/request"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// This test simulates the scenario mentioned in #31392:
// Having two linked container, renaming the target and bringing a replacement
// and then deleting and recreating the source container linked to the new target.
// This checks that "rename" updates source container correctly and doesn't set it to null.
func TestRenameLinkedContainer(t *testing.T) {
	defer setupTest(t)()
	ctx := context.Background()
	client := request.NewAPIClient(t)

	aID := runSimpleContainer(ctx, t, client, "a0")

	bID := runSimpleContainer(ctx, t, client, "b0", func(config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig) {
		hostConfig.Links = []string{"a0"}
	})

	err := client.ContainerRename(ctx, aID, "a1")
	require.NoError(t, err)

	runSimpleContainer(ctx, t, client, "a0")

	err = client.ContainerRemove(ctx, bID, types.ContainerRemoveOptions{Force: true})
	require.NoError(t, err)

	bID = runSimpleContainer(ctx, t, client, "b0", func(config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig) {
		hostConfig.Links = []string{"a0"}
	})

	inspect, err := client.ContainerInspect(ctx, bID)
	require.NoError(t, err)
	assert.Equal(t, []string{"/a0:/b0/a0"}, inspect.HostConfig.Links)
}
