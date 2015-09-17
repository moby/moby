package main

import (
	"runtime"

	"github.com/go-check/check"
)

func init() {
	// FIXME. Temporarily turning this off for Windows as GH16039 was breaking
	// Windows to Linux CI @icecrime
	if runtime.GOOS != "windows" {
		check.Suite(newDockerHubPullSuite())
	}
}

// DockerHubPullSuite provides a isolated daemon that doesn't have all the
// images that are baked into our 'global' test environment daemon (e.g.,
// busybox, httpserver, ...).
//
// We use it for pull tests where we want to start fresh. As part of this
// suite, all images are removed after each test.
type DockerHubPullSuite struct {
	DockerIsolatedDaemonSuite
}

// newDockerHubPullSuite returns a new instance of a DockerHubPullSuite.
func newDockerHubPullSuite() *DockerHubPullSuite {
	return &DockerHubPullSuite{
		DockerIsolatedDaemonSuite: DockerIsolatedDaemonSuite{
			ds: &DockerSuite{},
		},
	}
}

// SetUpTest declares that all tests of this suite require network.
func (s *DockerHubPullSuite) SetUpTest(c *check.C) {
	testRequires(c, Network)
	s.DockerIsolatedDaemonSuite.SetUpTest(c)
}
