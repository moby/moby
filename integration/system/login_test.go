package system // import "github.com/moby/moby/integration/system"

import (
	"context"
	"testing"

	"github.com/moby/moby/api/types"
	"github.com/moby/moby/integration/internal/requirement"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/skip"
)

// Test case for GitHub 22244
func TestLoginFailsWithBadCredentials(t *testing.T) {
	skip.If(t, !requirement.HasHubConnectivity(t))

	defer setupTest(t)()
	client := testEnv.APIClient()

	config := types.AuthConfig{
		Username: "no-user",
		Password: "no-password",
	}
	_, err := client.RegistryLogin(context.Background(), config)
	assert.Assert(t, err != nil)
	assert.Check(t, is.ErrorContains(err, "unauthorized: incorrect username or password"))
	assert.Check(t, is.ErrorContains(err, "https://registry-1.docker.io/v2/"))
}
