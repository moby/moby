package main

import (
	"github.com/go-check/check"
	"github.com/moby/moby/api/types"
	"github.com/moby/moby/client"
	"github.com/moby/moby/integration-cli/checker"
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
