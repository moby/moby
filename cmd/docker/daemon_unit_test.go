// +build daemon

package main

import (
	"testing"

	"github.com/docker/docker/pkg/testutil/assert"
	"github.com/go-check/check"
	"github.com/spf13/cobra"
)

// Hook up gocheck into the "go test" runner.
func Test(t *testing.T) { check.TestingT(t) }

type DockerSuite struct{}

var _ = check.Suite(&DockerSuite{})

func stubRun(cmd *cobra.Command, args []string) error {
	return nil
}

func (s *DockerSuite) TestDaemonCommandHelp(c *check.C) {
	cmd := newDaemonCommand()
	cmd.RunE = stubRun
	cmd.SetArgs([]string{"--help"})
	err := cmd.Execute()
	assert.NilError(c, err)
}

func (s *DockerSuite) TestDaemonCommand(c *check.C) {
	cmd := newDaemonCommand()
	cmd.RunE = stubRun
	cmd.SetArgs([]string{"--containerd", "/foo"})
	err := cmd.Execute()
	assert.NilError(c, err)
}
