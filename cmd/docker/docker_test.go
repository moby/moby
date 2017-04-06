package main

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/cli/command"
	"github.com/docker/docker/cli/debug"
	"github.com/docker/docker/pkg/testutil/assert"
)

func TestClientDebugEnabled(t *testing.T) {
	defer debug.Disable()

	cmd := newDockerCommand(&command.DockerCli{})
	cmd.Flags().Set("debug", "true")

	err := cmd.PersistentPreRunE(cmd, []string{})
	assert.NilError(t, err)
	assert.Equal(t, os.Getenv("DEBUG"), "1")
	assert.Equal(t, logrus.GetLevel(), logrus.DebugLevel)
}

func TestExitStatusForInvalidSubcommandWithHelpFlag(t *testing.T) {
	discard := ioutil.Discard
	cmd := newDockerCommand(command.NewDockerCli(os.Stdin, discard, discard))
	cmd.SetArgs([]string{"help", "invalid"})
	err := cmd.Execute()
	assert.Error(t, err, "unknown help topic: invalid")
}
