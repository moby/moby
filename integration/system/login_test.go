package system // import "github.com/docker/docker/integration/system"

import (
	"fmt"
	"testing"

	"github.com/docker/docker/api/types/registry"
	"github.com/docker/docker/integration/internal/requirement"
	registrypkg "github.com/docker/docker/registry"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/skip"
)

// Test case for GitHub 22244
func TestLoginFailsWithBadCredentials(t *testing.T) {
	skip.If(t, !requirement.HasHubConnectivity(t))

	ctx := setupTest(t)
	apiClient := testEnv.APIClient()

	_, err := apiClient.RegistryLogin(ctx, registry.AuthConfig{
		Username: "no-user",
		Password: "no-password",
	})
	assert.Assert(t, err != nil)
	assert.Check(t, is.ErrorContains(err, "unauthorized: incorrect username or password"))
	assert.Check(t, is.ErrorContains(err, fmt.Sprintf("https://%s/v2/", registrypkg.DefaultRegistryHost)))
}
