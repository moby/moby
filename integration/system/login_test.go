package system // import "github.com/docker/docker/integration/system"

import (
	"context"
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

	defer setupTest(t)()
	client := testEnv.APIClient()

	config := registry.AuthConfig{
		Username: "no-user",
		Password: "no-password",
	}
	_, err := client.RegistryLogin(context.Background(), config)
	assert.Assert(t, err != nil)
	assert.Check(t, is.ErrorContains(err, "unauthorized: incorrect username or password"))
	assert.Check(t, is.ErrorContains(err, fmt.Sprintf("https://%s/v2/", registrypkg.DefaultRegistryHost)))
}
