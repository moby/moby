// +build !daemon

package main

import (
	"testing"

	"github.com/docker/docker/pkg/testutil/assert"
	"github.com/go-check/check"
)

// Hook up gocheck into the "go test" runner.
func Test(t *testing.T) { check.TestingT(t) }

type DockerSuite struct{}

var _ = check.Suite(&DockerSuite{})

func (s *DockerSuite) TestDaemonCommand(c *check.C) {
	cmd := newDaemonCommand()
	cmd.SetArgs([]string{"--help"})
	err := cmd.Execute()

	assert.Error(c, err, "Please run `dockerd`")
}
