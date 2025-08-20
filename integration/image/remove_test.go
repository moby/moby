package image

import (
	"strings"
	"testing"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/containerd/platforms"

	containertypes "github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/image"
	"github.com/moby/moby/client"
	"github.com/moby/moby/v2/integration/internal/container"
	iimage "github.com/moby/moby/v2/integration/internal/image"
	"github.com/moby/moby/v2/internal/testutils/specialimage"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/skip"
)

func TestRemoveImageOrphaning(t *testing.T) {
	ctx := setupTest(t)

	apiClient := testEnv.APIClient()

	imgName := strings.ToLower(t.Name())

	// Create a container from busybox, and commit a small change so we have a new image
	cID1 := container.Create(ctx, t, apiClient, container.WithCmd(""))
	commitResp1, err := apiClient.ContainerCommit(ctx, cID1, containertypes.CommitOptions{
		Changes:   []string{`ENTRYPOINT ["true"]`},
		Reference: imgName,
	})
	assert.NilError(t, err)

	// verifies that reference now points to first image
	resp, err := apiClient.ImageInspect(ctx, imgName)
	assert.NilError(t, err)
	assert.Check(t, is.Equal(resp.ID, commitResp1.ID))

	// Create a container from created image, and commit a small change with same reference name
	cID2 := container.Create(ctx, t, apiClient, container.WithImage(imgName), container.WithCmd(""))
	commitResp2, err := apiClient.ContainerCommit(ctx, cID2, containertypes.CommitOptions{
		Changes:   []string{`LABEL Maintainer="Integration Tests"`},
		Reference: imgName,
	})
	assert.NilError(t, err)

	// verifies that reference now points to second image
	resp, err = apiClient.ImageInspect(ctx, imgName)
	assert.NilError(t, err)
	assert.Check(t, is.Equal(resp.ID, commitResp2.ID))

	// try to remove the image, should not error out.
	_, err = apiClient.ImageRemove(ctx, imgName, client.ImageRemoveOptions{})
	assert.NilError(t, err)

	// check if the first image is still there
	resp, err = apiClient.ImageInspect(ctx, commitResp1.ID)
	assert.NilError(t, err)
	assert.Check(t, is.Equal(resp.ID, commitResp1.ID))

	// check if the second image has been deleted
	_, err = apiClient.ImageInspect(ctx, commitResp2.ID)
	assert.Check(t, is.ErrorContains(err, "No such image:"))
}

func TestRemoveByDigest(t *testing.T) {
	skip.If(t, !testEnv.UsingSnapshotter(), "RepoDigests doesn't include tags when using graphdrivers")

	ctx := setupTest(t)
	apiClient := testEnv.APIClient()

	err := apiClient.ImageTag(ctx, "busybox", "test-remove-by-digest:latest")
	assert.NilError(t, err)

	inspect, err := apiClient.ImageInspect(ctx, "test-remove-by-digest")
	assert.NilError(t, err)

	id := ""
	for _, ref := range inspect.RepoDigests {
		if strings.Contains(ref, "test-remove-by-digest") {
			id = ref
			break
		}
	}
	assert.Assert(t, id != "")

	_, err = apiClient.ImageRemove(ctx, id, client.ImageRemoveOptions{})
	assert.NilError(t, err, "error removing %s", id)

	_, err = apiClient.ImageInspect(ctx, "busybox")
	assert.NilError(t, err, "busybox image got deleted")

	inspect, err = apiClient.ImageInspect(ctx, "test-remove-by-digest")
	assert.Check(t, is.ErrorType(err, cerrdefs.IsNotFound))
	assert.Check(t, is.DeepEqual(inspect, image.InspectResponse{}))
}

func TestRemoveWithPlatform(t *testing.T) {
	skip.If(t, !testEnv.UsingSnapshotter())

	ctx := setupTest(t)
	apiClient := testEnv.APIClient()

	imgName := strings.ToLower(t.Name()) + ":latest"

	platformHost := platforms.Normalize(ocispec.Platform{
		Architecture: testEnv.DaemonInfo.Architecture,
		OS:           testEnv.DaemonInfo.OSType,
	})
	someOtherPlatform := platforms.Platform{
		OS:           "other",
		Architecture: "some",
	}

	var imageIdx *ocispec.Index
	var descs []ocispec.Descriptor
	iimage.Load(ctx, t, apiClient, func(dir string) (*ocispec.Index, error) {
		idx, d, err := specialimage.MultiPlatform(dir, imgName, []ocispec.Platform{
			platformHost,
			{
				OS:           "linux",
				Architecture: "test", Variant: "1",
			},
			{
				OS:           "linux",
				Architecture: "test", Variant: "2",
			},
			someOtherPlatform,
		})
		descs = d
		imageIdx = idx
		return idx, err
	})
	_ = imageIdx

	for _, tc := range []struct {
		platform *ocispec.Platform
		deleted  ocispec.Descriptor
	}{
		{&platformHost, descs[0]},
		{&someOtherPlatform, descs[3]},
	} {
		resp, err := apiClient.ImageRemove(ctx, imgName, client.ImageRemoveOptions{
			Platforms: []ocispec.Platform{*tc.platform},
			Force:     true,
		})
		assert.NilError(t, err)
		assert.Check(t, is.Len(resp, 1))
		for _, r := range resp {
			assert.Check(t, is.Equal(r.Untagged, ""), "No image should be untagged")
		}
		checkPlatformDeleted(t, imageIdx, resp, tc.deleted)
	}

	// Delete the rest
	resp, err := apiClient.ImageRemove(ctx, imgName, client.ImageRemoveOptions{})
	assert.NilError(t, err)

	assert.Check(t, is.Len(resp, 2))
	assert.Check(t, is.Equal(resp[0].Untagged, imgName))
	assert.Check(t, is.Equal(resp[1].Deleted, imageIdx.Manifests[0].Digest.String()))
	// TODO: Should it also include platform-specific manifests?
}

func checkPlatformDeleted(t *testing.T, imageIdx *ocispec.Index, resp []image.DeleteResponse, mfstDesc ocispec.Descriptor) {
	for _, r := range resp {
		if r.Deleted != "" {
			if assert.Check(t, is.Equal(r.Deleted, mfstDesc.Digest.String())) {
				continue
			}
			if r.Deleted == imageIdx.Manifests[0].Digest.String() {
				t.Log("Root image was deleted, expected only platform:", platforms.FormatAll(*mfstDesc.Platform))
			}
		}
	}
}
