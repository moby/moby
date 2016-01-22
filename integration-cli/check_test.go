package main

import (
	"fmt"
	"testing"

	"github.com/docker/docker/pkg/reexec"
	"github.com/go-check/check"
)

func Test(t *testing.T) {
	reexec.Init() // This is required for external graphdriver tests

	if !isLocalDaemon {
		fmt.Println("INFO: Testing against a remote daemon")
	} else {
		fmt.Println("INFO: Testing against a local daemon")
	}

	check.TestingT(t)
}

func init() {
	check.Suite(&DockerSuite{})
}

type DockerSuite struct {
}

func (s *DockerSuite) TearDownTest(c *check.C) {
	unpauseAllContainers()
	deleteAllContainers()
	deleteAllImages()
	deleteAllVolumes()
	deleteAllNetworks()
}

func init() {
	check.Suite(&DockerRegistrySuite{
		ds: &DockerSuite{},
	})
}

type DockerRegistrySuite struct {
	ds  *DockerSuite
	reg *testRegistryV2
	d   *Daemon
}

func (s *DockerRegistrySuite) SetUpTest(c *check.C) {
	testRequires(c, DaemonIsLinux)
	s.reg = setupRegistry(c, false)
	s.d = NewDaemon(c)
}

func (s *DockerRegistrySuite) TearDownTest(c *check.C) {
	if s.reg != nil {
		s.reg.Close()
	}
	if s.d != nil {
		s.d.Stop()
	}
	s.ds.TearDownTest(c)
}

func init() {
	check.Suite(&DockerSchema1RegistrySuite{
		ds: &DockerSuite{},
	})
}

type DockerSchema1RegistrySuite struct {
	ds  *DockerSuite
	reg *testRegistryV2
	d   *Daemon
}

func (s *DockerSchema1RegistrySuite) SetUpTest(c *check.C) {
	testRequires(c, DaemonIsLinux)
	s.reg = setupRegistry(c, true)
	s.d = NewDaemon(c)
}

func (s *DockerSchema1RegistrySuite) TearDownTest(c *check.C) {
	if s.reg != nil {
		s.reg.Close()
	}
	if s.d != nil {
		s.d.Stop()
	}
	s.ds.TearDownTest(c)
}

func init() {
	check.Suite(&DockerDaemonSuite{
		ds: &DockerSuite{},
	})
}

type DockerDaemonSuite struct {
	ds *DockerSuite
	d  *Daemon
}

func (s *DockerDaemonSuite) SetUpTest(c *check.C) {
	testRequires(c, DaemonIsLinux)
	s.d = NewDaemon(c)
}

func (s *DockerDaemonSuite) TearDownTest(c *check.C) {
	testRequires(c, DaemonIsLinux)
	if s.d != nil {
		s.d.Stop()
	}
	s.ds.TearDownTest(c)
}

func init() {
	check.Suite(&DockerTrustSuite{
		ds: &DockerSuite{},
	})
}

type DockerTrustSuite struct {
	ds  *DockerSuite
	reg *testRegistryV2
	not *testNotary
}

func (s *DockerTrustSuite) SetUpTest(c *check.C) {
	s.reg = setupRegistry(c, false)
	s.not = setupNotary(c)
}

func (s *DockerTrustSuite) TearDownTest(c *check.C) {
	if s.reg != nil {
		s.reg.Close()
	}
	if s.not != nil {
		s.not.Close()
	}
	s.ds.TearDownTest(c)
}

type DockerRegistriesSuite struct {
	ds          *DockerSuite
	reg1        *testRegistryV2
	reg2        *testRegistryV2
	regWithAuth *testRegistryV2
	d           *Daemon
}

func (s *DockerRegistriesSuite) SetUpTest(c *check.C) {
	s.reg1 = setupRegistryAt(c, privateRegistryURL, false, false)
	s.reg2 = setupRegistryAt(c, privateRegistryURL2, false, false)
	s.regWithAuth = setupRegistryAt(c, privateRegistryURL3, false, true)
	s.d = NewDaemon(c)
}

func (s *DockerRegistriesSuite) TearDownTest(c *check.C) {
	if s.reg1 != nil {
		s.reg1.Close()
	}
	if s.reg2 != nil {
		s.reg2.Close()
	}
	if s.regWithAuth != nil {
		out, err := s.d.Cmd("logout", s.regWithAuth.url)
		c.Assert(err, check.IsNil, check.Commentf(out))
		s.regWithAuth.Close()
	}
	if s.d != nil {
		s.d.Stop()
	}
	s.ds.TearDownTest(c)
}

func init() {
	check.Suite(&DockerRegistriesSuite{
		ds: &DockerSuite{},
	})
}
