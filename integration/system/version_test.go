package system

import (
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/moby/moby/api/types/system"
	"github.com/moby/moby/client"
	"github.com/moby/moby/v2/internal/testutil/request"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestVersion(t *testing.T) {
	ctx := setupTest(t)
	apiClient := testEnv.APIClient()

	version, err := apiClient.ServerVersion(ctx, client.ServerVersionOptions{})
	assert.NilError(t, err)
	assert.Check(t, len(version.Components) > 0, "expected at least one component in version.Components")

	var engine system.ComponentVersion
	var found bool

	for _, comp := range version.Components {
		if comp.Name == "Engine" {
			engine = comp
			found = true
			break
		}
	}

	assert.Check(t, found, "Engine component not found in version.Components")
	assert.Equal(t, engine.Name, "Engine")
	assert.Check(t, engine.Version != "")
	assert.Equal(t, engine.Details["ApiVersion"], version.APIVersion)
	assert.Equal(t, engine.Details["MinAPIVersion"], version.MinAPIVersion)
	assert.Check(t, is.Equal(testEnv.DaemonInfo.OSType, engine.Details["Os"]))

	experimentalStr := engine.Details["Experimental"]
	experimentalBool, err := strconv.ParseBool(experimentalStr)
	assert.NilError(t, err, "Experimental field in Engine details is not a valid boolean string")
	assert.Equal(t, testEnv.DaemonInfo.ExperimentalBuild, experimentalBool)
}

func TestAPIClientVersionOldNotSupported(t *testing.T) {
	ctx := setupTest(t)
	minApiVersion := testEnv.DaemonMinAPIVersion
	major, minor, _ := strings.Cut(minApiVersion, ".")
	vMinInt, err := strconv.Atoi(minor)
	assert.NilError(t, err)
	vMinInt--
	version := fmt.Sprintf("%s.%d", major, vMinInt)
	apiClient := request.NewAPIClient(t, client.WithVersion(version))

	expectedErrorMessage := fmt.Sprintf("Error response from daemon: client version %s is too old. Minimum supported API version is %s, please upgrade your client to a newer version", version, minApiVersion)
	_, err = apiClient.ServerVersion(ctx, client.ServerVersionOptions{})
	assert.Error(t, err, expectedErrorMessage)
}
