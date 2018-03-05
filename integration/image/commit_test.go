package image // import "github.com/docker/docker/integration/image"

import (
	"context"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/integration/internal/container"
	"github.com/docker/docker/integration/internal/request"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCommitInheritsEnv(t *testing.T) {
	defer setupTest(t)()
	client := request.NewAPIClient(t)
	ctx := context.Background()

	cID1 := container.Create(t, ctx, client)

	commitResp1, err := client.ContainerCommit(ctx, cID1, types.ContainerCommitOptions{
		Changes:   []string{"ENV PATH=/bin"},
		Reference: "test-commit-image",
	})
	require.NoError(t, err)

	image1, _, err := client.ImageInspectWithRaw(ctx, commitResp1.ID)
	require.NoError(t, err)

	expectedEnv1 := []string{"PATH=/bin"}
	assert.Equal(t, expectedEnv1, image1.Config.Env)

	cID2 := container.Create(t, ctx, client, container.WithImage(image1.ID))

	commitResp2, err := client.ContainerCommit(ctx, cID2, types.ContainerCommitOptions{
		Changes:   []string{"ENV PATH=/usr/bin:$PATH"},
		Reference: "test-commit-image",
	})
	require.NoError(t, err)

	image2, _, err := client.ImageInspectWithRaw(ctx, commitResp2.ID)
	require.NoError(t, err)
	expectedEnv2 := []string{"PATH=/usr/bin:/bin"}
	assert.Equal(t, expectedEnv2, image2.Config.Env)
}
