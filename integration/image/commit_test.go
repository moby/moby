package image // import "github.com/docker/docker/integration/image"

import (
	"context"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/integration/internal/request"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCommitInheritsEnv(t *testing.T) {
	defer setupTest(t)()
	client := request.NewAPIClient(t)
	ctx := context.Background()

	createResp1, err := client.ContainerCreate(ctx, &container.Config{Image: "busybox"}, nil, nil, "")
	require.NoError(t, err)

	commitResp1, err := client.ContainerCommit(ctx, createResp1.ID, types.ContainerCommitOptions{
		Changes:   []string{"ENV PATH=/bin"},
		Reference: "test-commit-image",
	})
	require.NoError(t, err)

	image1, _, err := client.ImageInspectWithRaw(ctx, commitResp1.ID)
	require.NoError(t, err)

	expectedEnv1 := []string{"PATH=/bin"}
	assert.Equal(t, expectedEnv1, image1.Config.Env)

	createResp2, err := client.ContainerCreate(ctx, &container.Config{Image: image1.ID}, nil, nil, "")
	require.NoError(t, err)

	commitResp2, err := client.ContainerCommit(ctx, createResp2.ID, types.ContainerCommitOptions{
		Changes:   []string{"ENV PATH=/usr/bin:$PATH"},
		Reference: "test-commit-image",
	})
	require.NoError(t, err)

	image2, _, err := client.ImageInspectWithRaw(ctx, commitResp2.ID)
	require.NoError(t, err)
	expectedEnv2 := []string{"PATH=/usr/bin:/bin"}
	assert.Equal(t, expectedEnv2, image2.Config.Env)
}
