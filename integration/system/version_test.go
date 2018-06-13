package system // import "github.com/docker/docker/integration/system"

import (
	"context"
	"testing"

	"github.com/docker/docker/internal/test/request"
	"gotest.tools/assert"
	is "gotest.tools/assert/cmp"
)

func TestVersion(t *testing.T) {
	client := request.NewAPIClient(t)

	version, err := client.ServerVersion(context.Background())
	assert.NilError(t, err)

	assert.Check(t, version.APIVersion != "")
	assert.Check(t, version.Version != "")
	assert.Check(t, version.MinAPIVersion != "")
	assert.Check(t, is.Equal(testEnv.DaemonInfo.ExperimentalBuild, version.Experimental))
	assert.Check(t, is.Equal(testEnv.OSType, version.Os))
}
