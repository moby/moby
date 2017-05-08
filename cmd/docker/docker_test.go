package main

import (
	"os"
	"testing"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/utils"

	"github.com/docker/docker/api/client"
)

func TestClientDebugEnabled(t *testing.T) {
	defer utils.DisableDebug()

	cmd := newDockerCommand(&client.DockerCli{})
	cmd.Flags().Set("debug", "true")

	if err := cmd.PersistentPreRunE(cmd, []string{}); err != nil {
		t.Fatalf("Unexpected error: %s", err.Error())
	}

	if os.Getenv("DEBUG") != "1" {
		t.Fatal("expected debug enabled, got false")
	}
	if logrus.GetLevel() != logrus.DebugLevel {
		t.Fatalf("expected logrus debug level, got %v", logrus.GetLevel())
	}
}
