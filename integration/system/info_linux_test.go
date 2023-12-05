//go:build !windows

package system // import "github.com/docker/docker/integration/system"

import (
	"net/http"
	"testing"

	"github.com/docker/docker/testutil"
	req "github.com/docker/docker/testutil/request"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestInfoBinaryCommits(t *testing.T) {
	ctx := setupTest(t)
	client := testEnv.APIClient()

	info, err := client.Info(ctx)
	assert.NilError(t, err)

	assert.Check(t, "N/A" != info.ContainerdCommit.ID)
	assert.Check(t, is.Equal(info.ContainerdCommit.Expected, info.ContainerdCommit.ID))

	assert.Check(t, "N/A" != info.InitCommit.ID)
	assert.Check(t, is.Equal(info.InitCommit.Expected, info.InitCommit.ID))

	assert.Check(t, "N/A" != info.RuncCommit.ID)
	assert.Check(t, is.Equal(info.RuncCommit.Expected, info.RuncCommit.ID))
}

func TestInfoAPIVersioned(t *testing.T) {
	ctx := testutil.StartSpan(baseContext, t)
	// Windows only supports 1.25 or later

	res, body, err := req.Get(ctx, "/v1.24/info")
	assert.NilError(t, err)
	assert.Check(t, is.DeepEqual(res.StatusCode, http.StatusOK))

	b, err := req.ReadBody(body)
	assert.NilError(t, err)

	// Verify the old response on API 1.24 and older before commit
	// 6d98e344c7702a8a713cb9e02a19d83a79d3f930.
	out := string(b)
	assert.Check(t, is.Contains(out, "ExecutionDriver"))
	assert.Check(t, is.Contains(out, "not supported"))
}
