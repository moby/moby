package image // import "github.com/docker/docker/integration/image"

import (
	"context"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/integration/internal/container"
	"github.com/docker/docker/integration/internal/request"
	"github.com/docker/docker/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRemoveImageOrphaning(t *testing.T) {
	defer setupTest(t)()
	ctx := context.Background()
	client := request.NewAPIClient(t)

	img := "test-container-orphaning"

	// Create a container from busybox, and commit a small change so we have a new image
	cID1 := container.Create(t, ctx, client, container.WithCmd(""))
	commitResp1, err := client.ContainerCommit(ctx, cID1, types.ContainerCommitOptions{
		Changes:   []string{`ENTRYPOINT ["true"]`},
		Reference: img,
	})
	require.NoError(t, err)

	// verifies that reference now points to first image
	resp, _, err := client.ImageInspectWithRaw(ctx, img)
	require.NoError(t, err)
	assert.Equal(t, resp.ID, commitResp1.ID)

	// Create a container from created image, and commit a small change with same reference name
	cID2 := container.Create(t, ctx, client, container.WithImage(img), container.WithCmd(""))
	commitResp2, err := client.ContainerCommit(ctx, cID2, types.ContainerCommitOptions{
		Changes:   []string{`LABEL Maintainer="Integration Tests"`},
		Reference: img,
	})
	require.NoError(t, err)

	// verifies that reference now points to second image
	resp, _, err = client.ImageInspectWithRaw(ctx, img)
	require.NoError(t, err)
	assert.Equal(t, resp.ID, commitResp2.ID)

	// try to remove the image, should not error out.
	_, err = client.ImageRemove(ctx, img, types.ImageRemoveOptions{})
	require.NoError(t, err)

	// check if the first image is still there
	resp, _, err = client.ImageInspectWithRaw(ctx, commitResp1.ID)
	require.NoError(t, err)
	assert.Equal(t, resp.ID, commitResp1.ID)

	// check if the second image has been deleted
	_, _, err = client.ImageInspectWithRaw(ctx, commitResp2.ID)
	testutil.ErrorContains(t, err, "No such image:")
}
