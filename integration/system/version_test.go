package system // import "github.com/docker/docker/integration/system"

import (
	"testing"

	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestVersion(t *testing.T) {
	ctx := setupTest(t)
	apiClient := testEnv.APIClient()

	version, err := apiClient.ServerVersion(ctx)
	assert.NilError(t, err)

	assert.Check(t, version.APIVersion != "")
	assert.Check(t, version.Version != "")
	assert.Check(t, version.MinAPIVersion != "")
	assert.Check(t, is.Equal(testEnv.DaemonInfo.ExperimentalBuild, version.Experimental))
	assert.Check(t, is.Equal(testEnv.DaemonInfo.OSType, version.Os))
}
