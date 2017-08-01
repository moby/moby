package container

import (
	"context"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/strslice"
	"github.com/docker/docker/client"
	"github.com/docker/docker/integration/util/request"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func runContainer(ctx context.Context, t *testing.T, client client.APIClient, cntCfg *container.Config, hstCfg *container.HostConfig, nwkCfg *network.NetworkingConfig, cntName string) string {
	cnt, err := client.ContainerCreate(ctx, cntCfg, hstCfg, nwkCfg, cntName)
	require.NoError(t, err)

	err = client.ContainerStart(ctx, cnt.ID, types.ContainerStartOptions{})
	require.NoError(t, err)
	return cnt.ID
}

// This test simulates the scenario mentioned in #31392:
// Having two linked container, renaming the target and bringing a replacement
// and then deleting and recreating the source container linked to the new target.
// This checks that "rename" updates source container correctly and doesn't set it to null.
func TestRenameLinkedContainer(t *testing.T) {
	defer setupTest(t)()
	ctx := context.Background()
	client := request.NewAPIClient(t)

	cntConfig := &container.Config{
		Image: "busybox",
		Tty:   true,
		Cmd:   strslice.StrSlice([]string{"top"}),
	}

	var (
		aID, bID string
		cntJSON  types.ContainerJSON
		err      error
	)

	aID = runContainer(ctx, t, client,
		cntConfig,
		&container.HostConfig{},
		&network.NetworkingConfig{},
		"a0",
	)

	bID = runContainer(ctx, t, client,
		cntConfig,
		&container.HostConfig{
			Links: []string{"a0"},
		},
		&network.NetworkingConfig{},
		"b0",
	)

	err = client.ContainerRename(ctx, aID, "a1")
	require.NoError(t, err)

	runContainer(ctx, t, client,
		cntConfig,
		&container.HostConfig{},
		&network.NetworkingConfig{},
		"a0",
	)

	err = client.ContainerRemove(ctx, bID, types.ContainerRemoveOptions{Force: true})
	require.NoError(t, err)

	bID = runContainer(ctx, t, client,
		cntConfig,
		&container.HostConfig{
			Links: []string{"a0"},
		},
		&network.NetworkingConfig{},
		"b0",
	)

	cntJSON, err = client.ContainerInspect(ctx, bID)
	require.NoError(t, err)
	assert.Equal(t, []string{"/a0:/b0/a0"}, cntJSON.HostConfig.Links)
}
