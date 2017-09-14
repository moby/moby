package system

import (
	"testing"

	"github.com/docker/docker/integration-cli/request"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/context"
)

func TestVersion(t *testing.T) {
	client, err := request.NewClient()
	require.NoError(t, err)

	version, err := client.ServerVersion(context.Background())
	require.NoError(t, err)

	assert.NotNil(t, version.APIVersion)
	assert.NotNil(t, version.Version)
	assert.NotNil(t, version.MinAPIVersion)
	assert.Equal(t, testEnv.DaemonInfo.ExperimentalBuild, version.Experimental)
	assert.Equal(t, testEnv.DaemonInfo.OSType, version.Os)
}
