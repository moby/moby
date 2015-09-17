package main

import (
	"runtime"

	"github.com/go-check/check"
)

func init() {
	// FIXME. Temporarily turning this off for Windows as GH16039 was breaking
	// Windows to Linux CI @icecrime
	if runtime.GOOS != "windows" {
		check.Suite(newDockerRegistrySuite())
	}
}

// DockerRegistrySuite provides a isolated daemon that doesn't have all the
// images that are baked into our 'global' test environment daemon, except
// a busybox image for testing. It also provides a local registry to test
// against.
//
// We use it for push/pull tests against a local registry where we want to
// start fresh. As part of this suite, all images are removed after each test
// and busybox is restored).
type DockerRegistrySuite struct {
	DockerIsolatedDaemonSuite
	reg *testRegistryV2
}

// newDockerRegistrySuite returns a new instance of a DockerRegistrySuite.
func newDockerRegistrySuite() *DockerRegistrySuite {
	return &DockerRegistrySuite{
		DockerIsolatedDaemonSuite: DockerIsolatedDaemonSuite{
			ds: &DockerSuite{},
		},
	}
}

// SetUpTest declares that all tests of this suite require network, and
// sets up a local registry.
func (s *DockerRegistrySuite) SetUpTest(c *check.C) {
	testRequires(c, Network)
	s.reg = setupRegistry(c)
	s.DockerIsolatedDaemonSuite.SetUpTest(c)
	err := s.d.loadBusybox()
	c.Assert(err, check.IsNil)
}

// TearDownTest shuts down the local registry.
func (s *DockerRegistrySuite) TearDownTest(c *check.C) {
	if s.reg != nil {
		s.reg.Close()
	}
	s.DockerIsolatedDaemonSuite.TearDownTest(c)
}
