package system // import "github.com/docker/docker/integration/system"

import (
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/integration/internal/request"
	"github.com/docker/docker/integration/internal/requirement"
	"github.com/gotestyourself/gotestyourself/skip"
	"github.com/stretchr/testify/assert"
	"golang.org/x/net/context"
)

// Test case for GitHub 22244
func TestLoginFailsWithBadCredentials(t *testing.T) {
	skip.IfCondition(t, !requirement.HasHubConnectivity(t))

	client := request.NewAPIClient(t)

	config := types.AuthConfig{
		Username: "no-user",
		Password: "no-password",
	}
	_, err := client.RegistryLogin(context.Background(), config)
	expected := "Error response from daemon: Get https://registry-1.docker.io/v2/: unauthorized: incorrect username or password"
	assert.EqualError(t, err, expected)
}
