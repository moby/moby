package main

import (
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/docker/docker/integration-cli/checker"
	"github.com/go-check/check"
	"golang.org/x/net/context"
)

// Test case for #22244
func (s *DockerSuite) TestAuthAPI(c *check.C) {
	testRequires(c, Network)
	config := types.AuthConfig{
		Username: "no-user",
		Password: "no-password",
	}
	cli, err := client.NewEnvClient()
	c.Assert(err, checker.IsNil)
	defer cli.Close()

	_, err = cli.RegistryLogin(context.Background(), config)
	expected := "Get https://registry-1.docker.io/v2/: unauthorized: incorrect username or password"
	c.Assert(err.Error(), checker.Contains, expected)
}
