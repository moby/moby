// +build !windows

package system // import "github.com/docker/docker/integration/system"

import (
	"net/http"
	"testing"

	req "github.com/docker/docker/integration-cli/request"
	"github.com/docker/docker/integration/internal/request"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/context"
)

func TestInfoBinaryCommits(t *testing.T) {
	client := request.NewAPIClient(t)

	info, err := client.Info(context.Background())
	require.NoError(t, err)

	assert.NotNil(t, info.ContainerdCommit)
	assert.NotEqual(t, "N/A", info.ContainerdCommit.ID)
	assert.Equal(t, testEnv.DaemonInfo.ContainerdCommit.Expected, info.ContainerdCommit.Expected)
	assert.Equal(t, info.ContainerdCommit.Expected, info.ContainerdCommit.ID)

	assert.NotNil(t, info.InitCommit)
	assert.NotEqual(t, "N/A", info.InitCommit.ID)
	assert.Equal(t, testEnv.DaemonInfo.InitCommit.Expected, info.InitCommit.Expected)
	assert.Equal(t, info.InitCommit.Expected, info.InitCommit.ID)

	assert.NotNil(t, info.RuncCommit)
	assert.NotEqual(t, "N/A", info.RuncCommit.ID)
	assert.Equal(t, testEnv.DaemonInfo.RuncCommit.Expected, info.RuncCommit.Expected)
	assert.Equal(t, info.RuncCommit.Expected, info.RuncCommit.ID)
}

func TestInfoAPIVersioned(t *testing.T) {
	// Windows only supports 1.25 or later

	res, body, err := req.Get("/v1.20/info")
	require.NoError(t, err)
	assert.Equal(t, res.StatusCode, http.StatusOK)

	b, err := req.ReadBody(body)
	require.NoError(t, err)

	out := string(b)
	assert.Contains(t, out, "ExecutionDriver")
	assert.Contains(t, out, "not supported")
}
