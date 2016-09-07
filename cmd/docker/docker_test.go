package main

import (
	"os"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/utils"

	"github.com/docker/docker/api/client"
	"github.com/go-check/check"
)

func (s *DockerSuite) TestClientDebugEnabled(c *check.C) {
	defer utils.DisableDebug()

	cmd := newDockerCommand(&client.DockerCli{})
	cmd.Flags().Set("debug", "true")

	if err := cmd.PersistentPreRunE(cmd, []string{}); err != nil {
		c.Fatalf("Unexpected error: %s", err.Error())
	}

	if os.Getenv("DEBUG") != "1" {
		c.Fatal("expected debug enabled, got false")
	}
	if logrus.GetLevel() != logrus.DebugLevel {
		c.Fatalf("expected logrus debug level, got %v", logrus.GetLevel())
	}
}
