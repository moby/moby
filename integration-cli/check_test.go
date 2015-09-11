package main

import (
	"os/exec"
	"testing"

	"github.com/go-check/check"
)

// CmdMaker is an interface implemented by test suites. It's used so that
// testsuites which create their own daemon instance can make commands which
// target that daemon.
type CmdMaker interface {
	MakeCmd(arg ...string) *exec.Cmd
}

func Test(t *testing.T) {
	check.TestingT(t)
}

func init() {
	check.Suite(&DockerSuite{})
}

type DockerSuite struct {
}

func (s *DockerSuite) TearDownTest(c *check.C) {
	deleteAllContainers(s)
	deleteAllImages(s)
	deleteAllVolumes()
}

// MakeCmd returns a exec.Cmd command to run against the suite daemon.
func (s *DockerSuite) MakeCmd(arg ...string) *exec.Cmd {
	return exec.Command(dockerBinary, arg...)
}

func init() {
	check.Suite(&DockerDaemonSuite{
		DockerSuite: &DockerSuite{},
	})
}

type DockerDaemonSuite struct {
	*DockerSuite
	d *Daemon
}

func (s *DockerDaemonSuite) SetUpTest(c *check.C) {
	testRequires(c, DaemonIsLinux)
	s.d = NewDaemon(c)
}

func (s *DockerDaemonSuite) TearDownTest(c *check.C) {
	testRequires(c, DaemonIsLinux)
	s.d.Stop()
	s.DockerSuite.TearDownTest(c)
}
