package main

import (
	"os"
	"testing"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/utils"

	"github.com/docker/docker/api/client"
	cliflags "github.com/docker/docker/cli/flags"
)

func TestClientDebugEnabled(t *testing.T) {
	defer utils.DisableDebug()

	opts := cliflags.NewClientOptions()
	cmd := newDockerCommand(&client.DockerCli{}, opts)

	opts.Common.Debug = true
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
