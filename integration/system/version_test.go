package system

import (
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/moby/moby/client"
	"github.com/moby/moby/v2/testutil/request"
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

func TestAPIClientVersionOldNotSupported(t *testing.T) {
	ctx := setupTest(t)
	major, minor, _ := strings.Cut(testEnv.DaemonVersion.MinAPIVersion, ".")
	vMinInt, err := strconv.Atoi(minor)
	assert.NilError(t, err)
	vMinInt--
	version := fmt.Sprintf("%s.%d", major, vMinInt)
	apiClient := request.NewAPIClient(t, client.WithVersion(version))

	expectedErrorMessage := fmt.Sprintf("Error response from daemon: client version %s is too old. Minimum supported API version is %s, please upgrade your client to a newer version", version, testEnv.DaemonVersion.MinAPIVersion)
	_, err = apiClient.ServerVersion(ctx)
	assert.Error(t, err, expectedErrorMessage)
}
