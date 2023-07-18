package image // import "github.com/docker/docker/integration/image"

import (
	"context"
	"strings"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/integration/internal/container"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/skip"
)

func TestRemoveImageOrphaning(t *testing.T) {
	defer setupTest(t)()
	ctx := context.Background()
	client := testEnv.APIClient()

	imgName := strings.ToLower(t.Name())

	// Create a container from busybox, and commit a small change so we have a new image
	cID1 := container.Create(ctx, t, client, container.WithCmd(""))
	commitResp1, err := client.ContainerCommit(ctx, cID1, types.ContainerCommitOptions{
		Changes:   []string{`ENTRYPOINT ["true"]`},
		Reference: imgName,
	})
	assert.NilError(t, err)

	// verifies that reference now points to first image
	resp, _, err := client.ImageInspectWithRaw(ctx, imgName)
	assert.NilError(t, err)
	assert.Check(t, is.Equal(resp.ID, commitResp1.ID))

	// Create a container from created image, and commit a small change with same reference name
	cID2 := container.Create(ctx, t, client, container.WithImage(imgName), container.WithCmd(""))
	commitResp2, err := client.ContainerCommit(ctx, cID2, types.ContainerCommitOptions{
		Changes:   []string{`LABEL Maintainer="Integration Tests"`},
		Reference: imgName,
	})
	assert.NilError(t, err)

	// verifies that reference now points to second image
	resp, _, err = client.ImageInspectWithRaw(ctx, imgName)
	assert.NilError(t, err)
	assert.Check(t, is.Equal(resp.ID, commitResp2.ID))

	// try to remove the image, should not error out.
	_, err = client.ImageRemove(ctx, imgName, types.ImageRemoveOptions{})
	assert.NilError(t, err)

	// check if the first image is still there
	resp, _, err = client.ImageInspectWithRaw(ctx, commitResp1.ID)
	assert.NilError(t, err)
	assert.Check(t, is.Equal(resp.ID, commitResp1.ID))

	// check if the second image has been deleted
	_, _, err = client.ImageInspectWithRaw(ctx, commitResp2.ID)
	assert.Check(t, is.ErrorContains(err, "No such image:"))
}

func TestRemoveByDigest(t *testing.T) {
	skip.If(t, !testEnv.UsingSnapshotter(), "RepoDigests doesn't include tags when using graphdrivers")

	defer setupTest(t)()
	ctx := context.Background()
	client := testEnv.APIClient()

	err := client.ImageTag(ctx, "busybox", "test-remove-by-digest:latest")
	assert.NilError(t, err)

	inspect, _, err := client.ImageInspectWithRaw(ctx, "test-remove-by-digest")
	assert.NilError(t, err)

	id := ""
	for _, ref := range inspect.RepoDigests {
		if strings.Contains(ref, "test-remove-by-digest") {
			id = ref
			break
		}
	}
	assert.Assert(t, id != "")

	t.Logf("removing %s", id)
	_, err = client.ImageRemove(ctx, id, types.ImageRemoveOptions{})
	assert.NilError(t, err)

	inspect, _, err = client.ImageInspectWithRaw(ctx, "busybox")
	assert.Check(t, err, "busybox image got deleted")

	inspect, _, err = client.ImageInspectWithRaw(ctx, "test-remove-by-digest")
	assert.Check(t, is.ErrorType(err, errdefs.IsNotFound))
}
