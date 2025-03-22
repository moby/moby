package image

import (
	"bufio"
	"bytes"
	"io"
	"os"
	"testing"

	"github.com/containerd/platforms"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/versions"
	"github.com/docker/docker/client"
	"github.com/docker/docker/internal/testutils/specialimage"
	"github.com/docker/docker/pkg/jsonmessage"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/skip"
)

func TestLoadMultiplatform(t *testing.T) {
	// TODO(p1-0tr): figure out correct versions here
	skip.If(t, versions.LessThan(testEnv.DaemonAPIVersion(), "1.48"), "OCI layout support was introduced in v29")
	skip.If(t, !testEnv.UsingSnapshotter())

	ctx := setupTest(t)

	apiClient := testEnv.APIClient()

	tmp, err := os.MkdirTemp("", "integration-save-")
	assert.NilError(t, err)
	defer os.RemoveAll(tmp)

	testPlatforms := []ocispec.Platform{
		{OS: "linux", Architecture: "amd64"},
		{OS: "linux", Architecture: "arm64", Variant: "v8"},
	}
	imageRef := specialimage.Load(ctx, t, apiClient, func(dir string) (*ocispec.Index, error) {
		idx, _, err := specialimage.MultiPlatform(dir, "multiplatform:latest", testPlatforms)
		return idx, err
	})

	inspectPre, err := apiClient.ImageInspect(ctx, "multiplatform:latest", client.ImageInspectWithManifests(true))
	assert.NilError(t, err)

	rdr, err := apiClient.ImageSave(ctx, []string{imageRef}, client.ImageSaveWithPlatforms(testPlatforms...))
	assert.NilError(t, err)
	defer rdr.Close()

	tar, err := os.Create(tmp + "/image.tar")
	assert.NilError(t, err, "failed to create image tar file")
	defer tar.Close()

	_, err = io.Copy(tar, rdr)
	assert.NilError(t, err, "failed to write image tar file")

	tarFile, err := os.Open(tmp + "/image.tar")
	assert.NilError(t, err, "failed to open image tar file")
	defer tarFile.Close()

	_, err = apiClient.ImageRemove(ctx, imageRef, image.RemoveOptions{PruneChildren: true, Force: true})
	assert.NilError(t, err)

	imageRdr := bufio.NewReader(tarFile)

	ldr, err := apiClient.ImageLoad(ctx, imageRdr, client.ImageLoadWithPlatforms(testPlatforms[0]))
	assert.NilError(t, err)

	defer ldr.Body.Close()

	buf := bytes.NewBuffer(nil)
	err = jsonmessage.DisplayJSONMessagesStream(ldr.Body, buf, 0, false, nil)
	assert.NilError(t, err)

	inspectPost, err := apiClient.ImageInspect(ctx, "multiplatform:latest", client.ImageInspectWithManifests(true))
	assert.NilError(t, err)

	missingPlatformMatcher := platforms.Any(testPlatforms[1])

	assert.Equal(t, len(inspectPre.Manifests), len(inspectPost.Manifests))
	for i := range len(inspectPost.Manifests) {
		if missingPlatformMatcher.Match(*inspectPost.Manifests[i].Descriptor.Platform) {
			t.Logf("%v", inspectPost.Manifests[i].ImageData.Platform)
			assert.Equal(t, inspectPost.Manifests[i].Available, false)
		} else {
			assert.DeepEqual(t, inspectPre.Manifests[i], inspectPost.Manifests[i])
		}
	}
}
