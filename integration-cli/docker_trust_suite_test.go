package main

import (
	"runtime"

	"github.com/go-check/check"
)

func init() {
	// FIXME. Temporarily turning this off for Windows as GH16039 was breaking
	// Windows to Linux CI @icecrime
	if runtime.GOOS != "windows" {
		check.Suite(newDockerTrustSuite())
	}
}

// DockerTrustSuite is used for testing content trust with pushes and pulls.
// It combines a isolated daemon with a fresh busybox image, a local registry,
// and a notary server.
type DockerTrustSuite struct {
	DockerIsolatedDaemonSuite
	reg *testRegistryV2
	not *testNotary
}

// newDockerTrustSuite returns a new instance of a DockerTrustSuite.
func newDockerTrustSuite() *DockerTrustSuite {
	return &DockerTrustSuite{
		DockerIsolatedDaemonSuite: DockerIsolatedDaemonSuite{
			ds: &DockerSuite{},
		},
	}
}

// SetUpTest declares that all tests of this suite require network, and
// sets up a local registry and notary server.
func (s *DockerTrustSuite) SetUpTest(c *check.C) {
	testRequires(c, Network)
	s.reg = setupRegistry(c)
	s.not = setupNotary(c)
	s.DockerIsolatedDaemonSuite.SetUpTest(c)
	err := s.d.loadBusybox()
	c.Assert(err, check.IsNil)
}

// TearDownTest shuts down the local registry.
func (s *DockerTrustSuite) TearDownTest(c *check.C) {
	if s.reg != nil {
		s.reg.Close()
	}
	s.not.Close()
	s.DockerIsolatedDaemonSuite.TearDownTest(c)
}
