package image

import (
	"context"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/versions"
	"github.com/docker/docker/errdefs"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/skip"
)

func TestImagePullPlatformInvalid(t *testing.T) {
	skip.If(t, versions.LessThan(testEnv.DaemonAPIVersion(), "1.40"), "experimental in older versions")
	defer setupTest(t)()
	client := testEnv.APIClient()
	ctx := context.Background()

	_, err := client.ImagePull(ctx, "docker.io/library/hello-world:latest", types.ImagePullOptions{Platform: "foobar"})
	assert.Assert(t, err != nil)
	assert.ErrorContains(t, err, "unknown operating system or architecture")
	assert.Assert(t, errdefs.IsInvalidParameter(err))
}
