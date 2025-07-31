package main

import (
	"context"
	"os/exec"
	"strings"
	"testing"

	"github.com/moby/moby/v2/integration-cli/daemon"
	testdaemon "github.com/moby/moby/v2/testutil/daemon"
	"gotest.tools/v3/assert"
)

// DockerHubPullSuite provides an isolated daemon that doesn't have all the
// images that are baked into our 'global' test environment daemon (e.g.,
// busybox, httpserver, ...).
//
// We use it for push/pull tests where we want to start fresh, and measure the
// relative impact of each individual operation. As part of this suite, all
// images are removed after each test.
type DockerHubPullSuite struct {
	d  *daemon.Daemon
	ds *DockerSuite
}

// newDockerHubPullSuite returns a new instance of a DockerHubPullSuite.
func newDockerHubPullSuite() *DockerHubPullSuite {
	return &DockerHubPullSuite{
		ds: &DockerSuite{},
	}
}

// SetUpSuite starts the suite daemon.
func (s *DockerHubPullSuite) SetUpSuite(ctx context.Context, t *testing.T) {
	testRequires(t, DaemonIsLinux, testEnv.IsLocalDaemon)
	s.d = daemon.New(t, dockerBinary, dockerdBinary, testdaemon.WithEnvironment(testEnv.Execution))
	s.d.Start(t)
}

// TearDownSuite stops the suite daemon.
func (s *DockerHubPullSuite) TearDownSuite(ctx context.Context, t *testing.T) {
	if s.d != nil {
		s.d.Stop(t)
	}
}

// SetUpTest declares that all tests of this suite require network.
func (s *DockerHubPullSuite) SetUpTest(ctx context.Context, t *testing.T) {
	testRequires(t, Network)
}

// TearDownTest removes all images from the suite daemon.
func (s *DockerHubPullSuite) TearDownTest(ctx context.Context, t *testing.T) {
	out := s.Cmd(t, "images", "-aq")
	images := strings.Split(out, "\n")
	images = append([]string{"rmi", "-f"}, images...)
	s.d.Cmd(images...)
	s.ds.TearDownTest(ctx, t)
}

// Cmd executes a command against the suite daemon and returns the combined
// output. The function fails the test when the command returns an error.
func (s *DockerHubPullSuite) Cmd(t *testing.T, name string, arg ...string) string {
	out, err := s.CmdWithError(name, arg...)
	assert.Assert(t, err == nil, "%q failed with errors: %s, %v", strings.Join(arg, " "), out, err)
	return out
}

// CmdWithError executes a command against the suite daemon and returns the
// combined output as well as any error.
func (s *DockerHubPullSuite) CmdWithError(name string, arg ...string) (string, error) {
	c := s.MakeCmd(name, arg...)
	b, err := c.CombinedOutput()
	return string(b), err
}

// MakeCmd returns an exec.Cmd command to run against the suite daemon.
func (s *DockerHubPullSuite) MakeCmd(name string, arg ...string) *exec.Cmd {
	args := []string{"--host", s.d.Sock(), name}
	args = append(args, arg...)
	return exec.Command(dockerBinary, args...)
}
