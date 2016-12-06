package main

import (
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"testing"

	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/cliconfig"
	"github.com/docker/docker/integration-cli/daemon"
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

	if daemonPlatform == "linux" {
		ensureFrozenImagesLinux(t)
	}
	check.TestingT(t)
}

func init() {
	check.Suite(&DockerSuite{})
}

type DockerSuite struct {
}

func (s *DockerSuite) OnTimeout(c *check.C) {
	if daemonPid > 0 && isLocalDaemon {
		daemon.SignalDaemonDump(daemonPid)
	}
}

func (s *DockerSuite) TearDownTest(c *check.C) {
	unpauseAllContainers(c)
	deleteAllContainers(c)
	deleteAllImages(c)
	deleteAllVolumes(c)
	deleteAllNetworks(c)
	if daemonPlatform == "linux" {
		deleteAllPlugins(c)
	}
}

func init() {
	check.Suite(&DockerRegistrySuite{
		ds: &DockerSuite{},
	})
}

type DockerRegistrySuite struct {
	ds  *DockerSuite
	reg *testRegistryV2
	d   *daemon.Daemon
}

func (s *DockerRegistrySuite) OnTimeout(c *check.C) {
	s.d.DumpStackAndQuit()
}

func (s *DockerRegistrySuite) SetUpTest(c *check.C) {
	testRequires(c, DaemonIsLinux, RegistryHosting)
	s.reg = setupRegistry(c, false, "", "")
	s.d = daemon.New(c, dockerBinary, dockerdBinary, daemon.Config{
		Experimental: experimentalDaemon,
	})
}

func (s *DockerRegistrySuite) TearDownTest(c *check.C) {
	if s.reg != nil {
		s.reg.Close()
	}
	if s.d != nil {
		s.d.Stop(c)
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
	d   *daemon.Daemon
}

func (s *DockerSchema1RegistrySuite) OnTimeout(c *check.C) {
	s.d.DumpStackAndQuit()
}

func (s *DockerSchema1RegistrySuite) SetUpTest(c *check.C) {
	testRequires(c, DaemonIsLinux, RegistryHosting, NotArm64)
	s.reg = setupRegistry(c, true, "", "")
	s.d = daemon.New(c, dockerBinary, dockerdBinary, daemon.Config{
		Experimental: experimentalDaemon,
	})
}

func (s *DockerSchema1RegistrySuite) TearDownTest(c *check.C) {
	if s.reg != nil {
		s.reg.Close()
	}
	if s.d != nil {
		s.d.Stop(c)
	}
	s.ds.TearDownTest(c)
}

func init() {
	check.Suite(&DockerRegistryAuthHtpasswdSuite{
		ds: &DockerSuite{},
	})
}

type DockerRegistryAuthHtpasswdSuite struct {
	ds  *DockerSuite
	reg *testRegistryV2
	d   *daemon.Daemon
}

func (s *DockerRegistryAuthHtpasswdSuite) OnTimeout(c *check.C) {
	s.d.DumpStackAndQuit()
}

func (s *DockerRegistryAuthHtpasswdSuite) SetUpTest(c *check.C) {
	testRequires(c, DaemonIsLinux, RegistryHosting)
	s.reg = setupRegistry(c, false, "htpasswd", "")
	s.d = daemon.New(c, dockerBinary, dockerdBinary, daemon.Config{
		Experimental: experimentalDaemon,
	})
}

func (s *DockerRegistryAuthHtpasswdSuite) TearDownTest(c *check.C) {
	if s.reg != nil {
		out, err := s.d.Cmd("logout", privateRegistryURL)
		c.Assert(err, check.IsNil, check.Commentf(out))
		s.reg.Close()
	}
	if s.d != nil {
		s.d.Stop(c)
	}
	s.ds.TearDownTest(c)
}

func init() {
	check.Suite(&DockerRegistryAuthTokenSuite{
		ds: &DockerSuite{},
	})
}

type DockerRegistryAuthTokenSuite struct {
	ds  *DockerSuite
	reg *testRegistryV2
	d   *daemon.Daemon
}

func (s *DockerRegistryAuthTokenSuite) OnTimeout(c *check.C) {
	s.d.DumpStackAndQuit()
}

func (s *DockerRegistryAuthTokenSuite) SetUpTest(c *check.C) {
	testRequires(c, DaemonIsLinux, RegistryHosting)
	s.d = daemon.New(c, dockerBinary, dockerdBinary, daemon.Config{
		Experimental: experimentalDaemon,
	})
}

func (s *DockerRegistryAuthTokenSuite) TearDownTest(c *check.C) {
	if s.reg != nil {
		out, err := s.d.Cmd("logout", privateRegistryURL)
		c.Assert(err, check.IsNil, check.Commentf(out))
		s.reg.Close()
	}
	if s.d != nil {
		s.d.Stop(c)
	}
	s.ds.TearDownTest(c)
}

func (s *DockerRegistryAuthTokenSuite) setupRegistryWithTokenService(c *check.C, tokenURL string) {
	if s == nil {
		c.Fatal("registry suite isn't initialized")
	}
	s.reg = setupRegistry(c, false, "token", tokenURL)
}

func init() {
	check.Suite(&DockerDaemonSuite{
		ds: &DockerSuite{},
	})
}

type DockerDaemonSuite struct {
	ds *DockerSuite
	d  *daemon.Daemon
}

func (s *DockerDaemonSuite) OnTimeout(c *check.C) {
	s.d.DumpStackAndQuit()
}

func (s *DockerDaemonSuite) SetUpTest(c *check.C) {
	testRequires(c, DaemonIsLinux, SameHostDaemon)
	s.d = daemon.New(c, dockerBinary, dockerdBinary, daemon.Config{
		Experimental: experimentalDaemon,
	})
}

func (s *DockerDaemonSuite) TearDownTest(c *check.C) {
	testRequires(c, DaemonIsLinux, SameHostDaemon)
	if s.d != nil {
		s.d.Stop(c)
	}
	s.ds.TearDownTest(c)
}

func (s *DockerDaemonSuite) TearDownSuite(c *check.C) {
	filepath.Walk(daemon.SockRoot, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			// ignore errors here
			// not cleaning up sockets is not really an error
			return nil
		}
		if fi.Mode() == os.ModeSocket {
			syscall.Unlink(path)
		}
		return nil
	})
	os.RemoveAll(daemon.SockRoot)
}

const defaultSwarmPort = 2477

func init() {
	check.Suite(&DockerSwarmSuite{
		ds: &DockerSuite{},
	})
}

type DockerSwarmSuite struct {
	server      *httptest.Server
	ds          *DockerSuite
	daemons     []*daemon.Swarm
	daemonsLock sync.Mutex // protect access to daemons
	portIndex   int
}

func (s *DockerSwarmSuite) OnTimeout(c *check.C) {
	s.daemonsLock.Lock()
	defer s.daemonsLock.Unlock()
	for _, d := range s.daemons {
		d.DumpStackAndQuit()
	}
}

func (s *DockerSwarmSuite) SetUpTest(c *check.C) {
	testRequires(c, DaemonIsLinux)
}

func (s *DockerSwarmSuite) AddDaemon(c *check.C, joinSwarm, manager bool) *daemon.Swarm {
	d := &daemon.Swarm{
		Daemon: daemon.New(c, dockerBinary, dockerdBinary, daemon.Config{
			Experimental: experimentalDaemon,
		}),
		Port: defaultSwarmPort + s.portIndex,
	}
	d.ListenAddr = fmt.Sprintf("0.0.0.0:%d", d.Port)
	args := []string{"--iptables=false", "--swarm-default-advertise-addr=lo"} // avoid networking conflicts
	d.StartWithBusybox(c, args...)

	if joinSwarm == true {
		if len(s.daemons) > 0 {
			tokens := s.daemons[0].JoinTokens(c)
			token := tokens.Worker
			if manager {
				token = tokens.Manager
			}
			c.Assert(d.Join(swarm.JoinRequest{
				RemoteAddrs: []string{s.daemons[0].ListenAddr},
				JoinToken:   token,
			}), check.IsNil)
		} else {
			c.Assert(d.Init(swarm.InitRequest{}), check.IsNil)
		}
	}

	s.portIndex++
	s.daemonsLock.Lock()
	s.daemons = append(s.daemons, d)
	s.daemonsLock.Unlock()

	return d
}

func (s *DockerSwarmSuite) TearDownTest(c *check.C) {
	testRequires(c, DaemonIsLinux)
	s.daemonsLock.Lock()
	for _, d := range s.daemons {
		if d != nil {
			d.Stop(c)
			// FIXME(vdemeester) should be handled by SwarmDaemon ?
			// raft state file is quite big (64MB) so remove it after every test
			walDir := filepath.Join(d.Root, "swarm/raft/wal")
			if err := os.RemoveAll(walDir); err != nil {
				c.Logf("error removing %v: %v", walDir, err)
			}

			d.CleanupExecRoot(c)
		}
	}
	s.daemons = nil
	s.daemonsLock.Unlock()

	s.portIndex = 0
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
	testRequires(c, RegistryHosting, NotaryServerHosting)
	s.reg = setupRegistry(c, false, "", "")
	s.not = setupNotary(c)
}

func (s *DockerTrustSuite) TearDownTest(c *check.C) {
	if s.reg != nil {
		s.reg.Close()
	}
	if s.not != nil {
		s.not.Close()
	}

	// Remove trusted keys and metadata after test
	os.RemoveAll(filepath.Join(cliconfig.ConfigDir(), "trust"))
	s.ds.TearDownTest(c)
}

func init() {
	ds := &DockerSuite{}
	check.Suite(&DockerTrustedSwarmSuite{
		trustSuite: DockerTrustSuite{
			ds: ds,
		},
		swarmSuite: DockerSwarmSuite{
			ds: ds,
		},
	})
}

type DockerTrustedSwarmSuite struct {
	swarmSuite DockerSwarmSuite
	trustSuite DockerTrustSuite
	reg        *testRegistryV2
	not        *testNotary
}

func (s *DockerTrustedSwarmSuite) SetUpTest(c *check.C) {
	s.swarmSuite.SetUpTest(c)
	s.trustSuite.SetUpTest(c)
}

func (s *DockerTrustedSwarmSuite) TearDownTest(c *check.C) {
	s.trustSuite.TearDownTest(c)
	s.swarmSuite.TearDownTest(c)
}

func (s *DockerTrustedSwarmSuite) OnTimeout(c *check.C) {
	s.swarmSuite.OnTimeout(c)
}
