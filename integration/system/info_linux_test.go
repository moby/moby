// +build !windows

package system // import "github.com/docker/docker/integration/system"

import (
	"context"
	"net/http"
	"testing"

	"github.com/docker/docker/internal/test/request"
	req "github.com/docker/docker/internal/test/request"
	"gotest.tools/assert"
	is "gotest.tools/assert/cmp"
)

func TestInfoBinaryCommits(t *testing.T) {
	client := request.NewAPIClient(t)

	info, err := client.Info(context.Background())
	assert.NilError(t, err)

	assert.Check(t, "N/A" != info.ContainerdCommit.ID)
	assert.Check(t, is.Equal(testEnv.DaemonInfo.ContainerdCommit.Expected, info.ContainerdCommit.Expected))
	assert.Check(t, is.Equal(info.ContainerdCommit.Expected, info.ContainerdCommit.ID))

	assert.Check(t, "N/A" != info.InitCommit.ID)
	assert.Check(t, is.Equal(testEnv.DaemonInfo.InitCommit.Expected, info.InitCommit.Expected))
	assert.Check(t, is.Equal(info.InitCommit.Expected, info.InitCommit.ID))

	assert.Check(t, "N/A" != info.RuncCommit.ID)
	assert.Check(t, is.Equal(testEnv.DaemonInfo.RuncCommit.Expected, info.RuncCommit.Expected))
	assert.Check(t, is.Equal(info.RuncCommit.Expected, info.RuncCommit.ID))
}

func TestInfoAPIVersioned(t *testing.T) {
	// Windows only supports 1.25 or later

	res, body, err := req.Get("/v1.20/info")
	assert.NilError(t, err)
	assert.Check(t, is.DeepEqual(res.StatusCode, http.StatusOK))

	b, err := req.ReadBody(body)
	assert.NilError(t, err)

	out := string(b)
	assert.Check(t, is.Contains(out, "ExecutionDriver"))
	assert.Check(t, is.Contains(out, "not supported"))
}
