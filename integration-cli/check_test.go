package main

import (
	"fmt"
	"io/ioutil"
	"net/http/httptest"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/integration-cli/checker"
	"github.com/docker/docker/integration-cli/cli"
	"github.com/docker/docker/integration-cli/cli/build/fakestorage"
	"github.com/docker/docker/integration-cli/daemon"
	"github.com/docker/docker/integration-cli/environment"
	"github.com/docker/docker/integration-cli/fixtures/plugin"
	"github.com/docker/docker/integration-cli/registry"
	ienv "github.com/docker/docker/internal/test/environment"
	"github.com/docker/docker/pkg/reexec"
	"github.com/go-check/check"
	"golang.org/x/net/context"
)

const (
	// the private registry to use for tests
	privateRegistryURL = "127.0.0.1:5000"

	// path to containerd's ctr binary
	ctrBinary = "docker-containerd-ctr"

	// the docker daemon binary to use
	dockerdBinary = "dockerd"
)

var (
	testEnv *environment.Execution

	// the docker client binary to use
	dockerBinary = ""
)

func init() {
	var err error

	reexec.Init() // This is required for external graphdriver tests

	testEnv, err = environment.New()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func TestMain(m *testing.M) {
	dockerBinary = testEnv.DockerBinary()
	err := ienv.EnsureFrozenImagesLinux(&testEnv.Execution)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	testEnv.Print()
	os.Exit(m.Run())
}

func Test(t *testing.T) {
	cli.SetTestEnvironment(testEnv)
	fakestorage.SetTestEnvironment(&testEnv.Execution)
	ienv.ProtectAll(t, &testEnv.Execution)
	check.TestingT(t)
}

func init() {
	check.Suite(&DockerSuite{})
}

type DockerSuite struct {
}

func (s *DockerSuite) OnTimeout(c *check.C) {
	if testEnv.IsRemoteDaemon() {
		return
	}
	path := filepath.Join(os.Getenv("DEST"), "docker.pid")
	b, err := ioutil.ReadFile(path)
	if err != nil {
		c.Fatalf("Failed to get daemon PID from %s\n", path)
	}

	rawPid, err := strconv.ParseInt(string(b), 10, 32)
	if err != nil {
		c.Fatalf("Failed to parse pid from %s: %s\n", path, err)
	}

	daemonPid := int(rawPid)
	if daemonPid > 0 {
		daemon.SignalDaemonDump(daemonPid)
	}
}

func (s *DockerSuite) TearDownTest(c *check.C) {
	testEnv.Clean(c)
}

func init() {
	check.Suite(&DockerRegistrySuite{
		ds: &DockerSuite{},
	})
}

type DockerRegistrySuite struct {
	ds  *DockerSuite
	reg *registry.V2
	d   *daemon.Daemon
}

func (s *DockerRegistrySuite) OnTimeout(c *check.C) {
	s.d.DumpStackAndQuit()
}

func (s *DockerRegistrySuite) SetUpTest(c *check.C) {
	testRequires(c, DaemonIsLinux, registry.Hosting, SameHostDaemon)
	s.reg = setupRegistry(c, false, "", "")
	s.d = daemon.New(c, dockerBinary, dockerdBinary, daemon.Config{
		Experimental: testEnv.DaemonInfo.ExperimentalBuild,
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
	reg *registry.V2
	d   *daemon.Daemon
}

func (s *DockerSchema1RegistrySuite) OnTimeout(c *check.C) {
	s.d.DumpStackAndQuit()
}

func (s *DockerSchema1RegistrySuite) SetUpTest(c *check.C) {
	testRequires(c, DaemonIsLinux, registry.Hosting, NotArm64, SameHostDaemon)
	s.reg = setupRegistry(c, true, "", "")
	s.d = daemon.New(c, dockerBinary, dockerdBinary, daemon.Config{
		Experimental: testEnv.DaemonInfo.ExperimentalBuild,
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
	reg *registry.V2
	d   *daemon.Daemon
}

func (s *DockerRegistryAuthHtpasswdSuite) OnTimeout(c *check.C) {
	s.d.DumpStackAndQuit()
}

func (s *DockerRegistryAuthHtpasswdSuite) SetUpTest(c *check.C) {
	testRequires(c, DaemonIsLinux, registry.Hosting, SameHostDaemon)
	s.reg = setupRegistry(c, false, "htpasswd", "")
	s.d = daemon.New(c, dockerBinary, dockerdBinary, daemon.Config{
		Experimental: testEnv.DaemonInfo.ExperimentalBuild,
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
	reg *registry.V2
	d   *daemon.Daemon
}

func (s *DockerRegistryAuthTokenSuite) OnTimeout(c *check.C) {
	s.d.DumpStackAndQuit()
}

func (s *DockerRegistryAuthTokenSuite) SetUpTest(c *check.C) {
	testRequires(c, DaemonIsLinux, registry.Hosting, SameHostDaemon)
	s.d = daemon.New(c, dockerBinary, dockerdBinary, daemon.Config{
		Experimental: testEnv.DaemonInfo.ExperimentalBuild,
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
		Experimental: testEnv.DaemonInfo.ExperimentalBuild,
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
	testRequires(c, DaemonIsLinux, SameHostDaemon)
}

func (s *DockerSwarmSuite) AddDaemon(c *check.C, joinSwarm, manager bool) *daemon.Swarm {
	d := &daemon.Swarm{
		Daemon: daemon.New(c, dockerBinary, dockerdBinary, daemon.Config{
			Experimental: testEnv.DaemonInfo.ExperimentalBuild,
		}),
		Port: defaultSwarmPort + s.portIndex,
	}
	d.ListenAddr = fmt.Sprintf("0.0.0.0:%d", d.Port)
	args := []string{"--iptables=false", "--swarm-default-advertise-addr=lo"} // avoid networking conflicts
	d.StartWithBusybox(c, args...)

	if joinSwarm {
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
	check.Suite(&DockerPluginSuite{
		ds: &DockerSuite{},
	})
}

type DockerPluginSuite struct {
	ds       *DockerSuite
	registry *registry.V2
}

func (ps *DockerPluginSuite) registryHost() string {
	return privateRegistryURL
}

func (ps *DockerPluginSuite) getPluginRepo() string {
	return path.Join(ps.registryHost(), "plugin", "basic")
}
func (ps *DockerPluginSuite) getPluginRepoWithTag() string {
	return ps.getPluginRepo() + ":" + "latest"
}

func (ps *DockerPluginSuite) SetUpSuite(c *check.C) {
	testRequires(c, DaemonIsLinux, registry.Hosting)
	ps.registry = setupRegistry(c, false, "", "")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	err := plugin.CreateInRegistry(ctx, ps.getPluginRepo(), nil)
	c.Assert(err, checker.IsNil, check.Commentf("failed to create plugin"))
}

func (ps *DockerPluginSuite) TearDownSuite(c *check.C) {
	if ps.registry != nil {
		ps.registry.Close()
	}
}

func (ps *DockerPluginSuite) TearDownTest(c *check.C) {
	ps.ds.TearDownTest(c)
}

func (ps *DockerPluginSuite) OnTimeout(c *check.C) {
	ps.ds.OnTimeout(c)
}
