// +build !windows

package system

import (
	"testing"

	"github.com/docker/docker/integration/util/request"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/context"
)

func TestInfo_BinaryCommits(t *testing.T) {
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
