package main

import (
	"os/exec"
	"strings"

	"github.com/go-check/check"
)

// DockerIsolatedDaemonSuite provides a isolated daemon that doesn't have all the
// images that are baked into our 'global' test environment daemon (e.g.,
// busybox, httpserver, ...).
//
// We use it as a basis for other testsuites to run for push/pull tests where we want to
// start fresh, and measure the relative impact of each individual operation. As part of
// this suite, all images are removed after each test.
type DockerIsolatedDaemonSuite struct {
	d  *Daemon
	ds *DockerSuite
}

// SetUpSuite starts the suite daemon.
func (s *DockerIsolatedDaemonSuite) SetUpSuite(c *check.C) {
	testRequires(c, DaemonIsLinux)
	s.d = NewDaemon(c)
	if err := s.d.Start(); err != nil {
		c.Fatalf("starting push/pull test daemon: %v", err)
	}
}

// TearDownSuite stops the suite daemon.
func (s *DockerIsolatedDaemonSuite) TearDownSuite(c *check.C) {
	if s.d != nil {
		if err := s.d.Stop(); err != nil {
			c.Fatalf("stopping push/pull test daemon: %v", err)
		}
	}
}

// SetUpTest is an empty function provided for consistency with TearDownTest.
func (s *DockerIsolatedDaemonSuite) SetUpTest(c *check.C) {
}

// TearDownTest removes all images from the suite daemon.
func (s *DockerIsolatedDaemonSuite) TearDownTest(c *check.C) {
	out := s.Cmd(c, "images", "-aq")
	images := strings.Split(out, "\n")
	images = append([]string{"-f"}, images...)
	s.d.Cmd("rmi", images...)
	s.ds.TearDownTest(c)
}

// Cmd executes a command against the suite daemon and returns the combined
// output. The function fails the test when the command returns an error.
func (s *DockerIsolatedDaemonSuite) Cmd(c *check.C, name string, arg ...string) string {
	out, err := s.CmdWithError(name, arg...)
	c.Assert(err, check.IsNil, check.Commentf("%q failed with errors: %s, %v", strings.Join(arg, " "), out, err))
	return out
}

// CmdWithError executes a command against the suite daemon and returns the
// combined output as well as any error.
func (s *DockerIsolatedDaemonSuite) CmdWithError(name string, arg ...string) (string, error) {
	c := s.MakeCmd(append([]string{name}, arg...)...)
	b, err := c.CombinedOutput()
	return string(b), err
}

// MakeCmd returns a exec.Cmd command to run against the suite daemon.
func (s *DockerIsolatedDaemonSuite) MakeCmd(arg ...string) *exec.Cmd {
	args := []string{"--host", s.d.sock()}
	args = append(args, arg...)
	return exec.Command(dockerBinary, args...)
}
