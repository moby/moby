package main

import (
	"context"
	"flag"
	"fmt"
	"net/http/httptest"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/docker/docker/integration-cli/cli"
	"github.com/docker/docker/integration-cli/daemon"
	"github.com/docker/docker/integration-cli/environment"
	"github.com/docker/docker/internal/test/suite"
	testdaemon "github.com/docker/docker/testutil/daemon"
	ienv "github.com/docker/docker/testutil/environment"
	"github.com/docker/docker/testutil/fakestorage"
	"github.com/docker/docker/testutil/fixtures/plugin"
	"github.com/docker/docker/testutil/registry"
	"gotest.tools/v3/assert"
)

const (
	// the private registry to use for tests
	privateRegistryURL = registry.DefaultURL

	// path to containerd's ctr binary
	ctrBinary = "ctr"

	// the docker daemon binary to use
	dockerdBinary = "dockerd"
)

var (
	testEnv *environment.Execution

	// the docker client binary to use
	dockerBinary = ""

	testEnvOnce sync.Once
)

func init() {
	var err error

	testEnv, err = environment.New()
	if err != nil {
		panic(err)
	}
}

func TestMain(m *testing.M) {
	flag.Parse()

	// Global set up
	dockerBinary = testEnv.DockerBinary()
	err := ienv.EnsureFrozenImagesLinux(&testEnv.Execution)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	testEnv.Print()
	printCliVersion()
	os.Exit(m.Run())
}

func printCliVersion() {
	// Print output of "docker version"
	cli.SetTestEnvironment(testEnv)
	cmd := cli.Docker(cli.Args("version"))
	if cmd.Error != nil {
		fmt.Printf("WARNING: Failed to run \"docker version\": %+v\n", cmd.Error)
		return
	}

	fmt.Println("INFO: Testing with docker cli version:")
	fmt.Println(cmd.Stdout())
}

func ensureTestEnvSetup(t *testing.T) {
	testEnvOnce.Do(func() {
		cli.SetTestEnvironment(testEnv)
		fakestorage.SetTestEnvironment(&testEnv.Execution)
		ienv.ProtectAll(t, &testEnv.Execution)
	})
}

func TestDockerAPISuite(t *testing.T) {
	ensureTestEnvSetup(t)
	suite.Run(t, &DockerAPISuite{ds: &DockerSuite{}})
}

func TestDockerBenchmarkSuite(t *testing.T) {
	ensureTestEnvSetup(t)
	suite.Run(t, &DockerBenchmarkSuite{ds: &DockerSuite{}})
}

func TestDockerCLIAttachSuite(t *testing.T) {
	ensureTestEnvSetup(t)
	suite.Run(t, &DockerCLIAttachSuite{ds: &DockerSuite{}})
}

func TestDockerCLIBuildSuite(t *testing.T) {
	ensureTestEnvSetup(t)
	suite.Run(t, &DockerCLIBuildSuite{ds: &DockerSuite{}})
}

func TestDockerCLICommitSuite(t *testing.T) {
	ensureTestEnvSetup(t)
	suite.Run(t, &DockerCLICommitSuite{ds: &DockerSuite{}})
}

func TestDockerCLICpSuite(t *testing.T) {
	ensureTestEnvSetup(t)
	suite.Run(t, &DockerCLICpSuite{ds: &DockerSuite{}})
}

func TestDockerCLICreateSuite(t *testing.T) {
	ensureTestEnvSetup(t)
	suite.Run(t, &DockerCLICreateSuite{ds: &DockerSuite{}})
}

func TestDockerCLIEventSuite(t *testing.T) {
	ensureTestEnvSetup(t)
	suite.Run(t, &DockerCLIEventSuite{ds: &DockerSuite{}})
}

func TestDockerCLIExecSuite(t *testing.T) {
	ensureTestEnvSetup(t)
	suite.Run(t, &DockerCLIExecSuite{ds: &DockerSuite{}})
}

func TestDockerCLIHealthSuite(t *testing.T) {
	ensureTestEnvSetup(t)
	suite.Run(t, &DockerCLIHealthSuite{ds: &DockerSuite{}})
}

func TestDockerCLIHistorySuite(t *testing.T) {
	ensureTestEnvSetup(t)
	suite.Run(t, &DockerCLIHistorySuite{ds: &DockerSuite{}})
}

func TestDockerCLIImagesSuite(t *testing.T) {
	ensureTestEnvSetup(t)
	suite.Run(t, &DockerCLIImagesSuite{ds: &DockerSuite{}})
}

func TestDockerCLIImportSuite(t *testing.T) {
	ensureTestEnvSetup(t)
	suite.Run(t, &DockerCLIImportSuite{ds: &DockerSuite{}})
}

func TestDockerCLIInfoSuite(t *testing.T) {
	ensureTestEnvSetup(t)
	suite.Run(t, &DockerCLIInfoSuite{ds: &DockerSuite{}})
}

func TestDockerCLIInspectSuite(t *testing.T) {
	ensureTestEnvSetup(t)
	suite.Run(t, &DockerCLIInspectSuite{ds: &DockerSuite{}})
}

func TestDockerCLILinksSuite(t *testing.T) {
	ensureTestEnvSetup(t)
	suite.Run(t, &DockerCLILinksSuite{ds: &DockerSuite{}})
}

func TestDockerCLILoginSuite(t *testing.T) {
	ensureTestEnvSetup(t)
	suite.Run(t, &DockerCLILoginSuite{ds: &DockerSuite{}})
}

func TestDockerCLILogsSuite(t *testing.T) {
	ensureTestEnvSetup(t)
	suite.Run(t, &DockerCLILogsSuite{ds: &DockerSuite{}})
}

func TestDockerCLINetmodeSuite(t *testing.T) {
	ensureTestEnvSetup(t)
	suite.Run(t, &DockerCLINetmodeSuite{ds: &DockerSuite{}})
}

func TestDockerCLINetworkSuite(t *testing.T) {
	ensureTestEnvSetup(t)
	suite.Run(t, &DockerCLINetworkSuite{ds: &DockerSuite{}})
}

func TestDockerCLIPluginLogDriverSuite(t *testing.T) {
	ensureTestEnvSetup(t)
	suite.Run(t, &DockerCLIPluginLogDriverSuite{ds: &DockerSuite{}})
}

func TestDockerCLIPluginsSuite(t *testing.T) {
	ensureTestEnvSetup(t)
	suite.Run(t, &DockerCLIPluginsSuite{ds: &DockerSuite{}})
}

func TestDockerCLIPortSuite(t *testing.T) {
	ensureTestEnvSetup(t)
	suite.Run(t, &DockerCLIPortSuite{ds: &DockerSuite{}})
}

func TestDockerCLIProxySuite(t *testing.T) {
	ensureTestEnvSetup(t)
	suite.Run(t, &DockerCLIProxySuite{ds: &DockerSuite{}})
}

func TestDockerCLIPruneSuite(t *testing.T) {
	ensureTestEnvSetup(t)
	suite.Run(t, &DockerCLIPruneSuite{ds: &DockerSuite{}})
}

func TestDockerCLIPsSuite(t *testing.T) {
	ensureTestEnvSetup(t)
	suite.Run(t, &DockerCLIPsSuite{ds: &DockerSuite{}})
}

func TestDockerCLIPullSuite(t *testing.T) {
	ensureTestEnvSetup(t)
	suite.Run(t, &DockerCLIPullSuite{ds: &DockerSuite{}})
}

func TestDockerCLIPushSuite(t *testing.T) {
	ensureTestEnvSetup(t)
	suite.Run(t, &DockerCLIPushSuite{ds: &DockerSuite{}})
}

func TestDockerCLIRestartSuite(t *testing.T) {
	ensureTestEnvSetup(t)
	suite.Run(t, &DockerCLIRestartSuite{ds: &DockerSuite{}})
}

func TestDockerCLIRmiSuite(t *testing.T) {
	ensureTestEnvSetup(t)
	suite.Run(t, &DockerCLIRmiSuite{ds: &DockerSuite{}})
}

func TestDockerCLIRunSuite(t *testing.T) {
	ensureTestEnvSetup(t)
	suite.Run(t, &DockerCLIRunSuite{ds: &DockerSuite{}})
}

func TestDockerCLISaveLoadSuite(t *testing.T) {
	ensureTestEnvSetup(t)
	suite.Run(t, &DockerCLISaveLoadSuite{ds: &DockerSuite{}})
}

func TestDockerCLISearchSuite(t *testing.T) {
	ensureTestEnvSetup(t)
	suite.Run(t, &DockerCLISearchSuite{ds: &DockerSuite{}})
}

func TestDockerCLISNISuite(t *testing.T) {
	ensureTestEnvSetup(t)
	suite.Run(t, &DockerCLISNISuite{ds: &DockerSuite{}})
}

func TestDockerCLIStartSuite(t *testing.T) {
	ensureTestEnvSetup(t)
	suite.Run(t, &DockerCLIStartSuite{ds: &DockerSuite{}})
}

func TestDockerCLIStatsSuite(t *testing.T) {
	ensureTestEnvSetup(t)
	suite.Run(t, &DockerCLIStatsSuite{ds: &DockerSuite{}})
}

func TestDockerCLITopSuite(t *testing.T) {
	ensureTestEnvSetup(t)
	suite.Run(t, &DockerCLITopSuite{ds: &DockerSuite{}})
}

func TestDockerCLIUpdateSuite(t *testing.T) {
	ensureTestEnvSetup(t)
	suite.Run(t, &DockerCLIUpdateSuite{ds: &DockerSuite{}})
}

func TestDockerCLIVolumeSuite(t *testing.T) {
	ensureTestEnvSetup(t)
	suite.Run(t, &DockerCLIVolumeSuite{ds: &DockerSuite{}})
}

func TestDockerRegistrySuite(t *testing.T) {
	ensureTestEnvSetup(t)
	suite.Run(t, &DockerRegistrySuite{ds: &DockerSuite{}})
}

func TestDockerSchema1RegistrySuite(t *testing.T) {
	ensureTestEnvSetup(t)
	suite.Run(t, &DockerSchema1RegistrySuite{ds: &DockerSuite{}})
}

func TestDockerRegistryAuthHtpasswdSuite(t *testing.T) {
	ensureTestEnvSetup(t)
	suite.Run(t, &DockerRegistryAuthHtpasswdSuite{ds: &DockerSuite{}})
}

func TestDockerRegistryAuthTokenSuite(t *testing.T) {
	ensureTestEnvSetup(t)
	suite.Run(t, &DockerRegistryAuthTokenSuite{ds: &DockerSuite{}})
}

func TestDockerDaemonSuite(t *testing.T) {
	ensureTestEnvSetup(t)
	suite.Run(t, &DockerDaemonSuite{ds: &DockerSuite{}})
}

func TestDockerSwarmSuite(t *testing.T) {
	ensureTestEnvSetup(t)
	suite.Run(t, &DockerSwarmSuite{ds: &DockerSuite{}})
}

func TestDockerPluginSuite(t *testing.T) {
	ensureTestEnvSetup(t)
	suite.Run(t, &DockerPluginSuite{ds: &DockerSuite{}})
}

func TestDockerExternalVolumeSuite(t *testing.T) {
	ensureTestEnvSetup(t)
	testRequires(t, DaemonIsLinux)
	suite.Run(t, &DockerExternalVolumeSuite{ds: &DockerSuite{}})
}

func TestDockerNetworkSuite(t *testing.T) {
	ensureTestEnvSetup(t)
	testRequires(t, DaemonIsLinux)
	suite.Run(t, &DockerNetworkSuite{ds: &DockerSuite{}})
}

func TestDockerHubPullSuite(t *testing.T) {
	ensureTestEnvSetup(t)
	// FIXME. Temporarily turning this off for Windows as GH16039 was breaking
	// Windows to Linux CI @icecrime
	testRequires(t, DaemonIsLinux)
	suite.Run(t, newDockerHubPullSuite())
}

type DockerSuite struct{}

func (s *DockerSuite) OnTimeout(c *testing.T) {
	if testEnv.IsRemoteDaemon() {
		return
	}
	path := filepath.Join(os.Getenv("DEST"), "docker.pid")
	b, err := os.ReadFile(path)
	if err != nil {
		c.Fatalf("Failed to get daemon PID from %s\n", path)
	}

	rawPid, err := strconv.ParseInt(string(b), 10, 32)
	if err != nil {
		c.Fatalf("Failed to parse pid from %s: %s\n", path, err)
	}

	daemonPid := int(rawPid)
	if daemonPid > 0 {
		testdaemon.SignalDaemonDump(daemonPid)
	}
}

func (s *DockerSuite) TearDownTest(c *testing.T) {
	testEnv.Clean(c)
}

type DockerRegistrySuite struct {
	ds  *DockerSuite
	reg *registry.V2
	d   *daemon.Daemon
}

func (s *DockerRegistrySuite) OnTimeout(c *testing.T) {
	s.d.DumpStackAndQuit()
}

func (s *DockerRegistrySuite) SetUpTest(c *testing.T) {
	testRequires(c, DaemonIsLinux, RegistryHosting, testEnv.IsLocalDaemon)
	s.reg = registry.NewV2(c)
	s.reg.WaitReady(c)
	s.d = daemon.New(c, dockerBinary, dockerdBinary, testdaemon.WithEnvironment(testEnv.Execution))
}

func (s *DockerRegistrySuite) TearDownTest(c *testing.T) {
	if s.reg != nil {
		s.reg.Close()
	}
	if s.d != nil {
		s.d.Stop(c)
	}
	s.ds.TearDownTest(c)
}

type DockerSchema1RegistrySuite struct {
	ds  *DockerSuite
	reg *registry.V2
	d   *daemon.Daemon
}

func (s *DockerSchema1RegistrySuite) OnTimeout(c *testing.T) {
	s.d.DumpStackAndQuit()
}

func (s *DockerSchema1RegistrySuite) SetUpTest(c *testing.T) {
	testRequires(c, DaemonIsLinux, RegistryHosting, NotArm64, testEnv.IsLocalDaemon)
	s.reg = registry.NewV2(c, registry.Schema1)
	s.reg.WaitReady(c)
	s.d = daemon.New(c, dockerBinary, dockerdBinary, testdaemon.WithEnvironment(testEnv.Execution))
}

func (s *DockerSchema1RegistrySuite) TearDownTest(c *testing.T) {
	if s.reg != nil {
		s.reg.Close()
	}
	if s.d != nil {
		s.d.Stop(c)
	}
	s.ds.TearDownTest(c)
}

type DockerRegistryAuthHtpasswdSuite struct {
	ds  *DockerSuite
	reg *registry.V2
	d   *daemon.Daemon
}

func (s *DockerRegistryAuthHtpasswdSuite) OnTimeout(c *testing.T) {
	s.d.DumpStackAndQuit()
}

func (s *DockerRegistryAuthHtpasswdSuite) SetUpTest(c *testing.T) {
	testRequires(c, DaemonIsLinux, RegistryHosting, testEnv.IsLocalDaemon)
	s.reg = registry.NewV2(c, registry.Htpasswd)
	s.reg.WaitReady(c)
	s.d = daemon.New(c, dockerBinary, dockerdBinary, testdaemon.WithEnvironment(testEnv.Execution))
}

func (s *DockerRegistryAuthHtpasswdSuite) TearDownTest(c *testing.T) {
	if s.reg != nil {
		out, err := s.d.Cmd("logout", privateRegistryURL)
		assert.NilError(c, err, out)
		s.reg.Close()
	}
	if s.d != nil {
		s.d.Stop(c)
	}
	s.ds.TearDownTest(c)
}

type DockerRegistryAuthTokenSuite struct {
	ds  *DockerSuite
	reg *registry.V2
	d   *daemon.Daemon
}

func (s *DockerRegistryAuthTokenSuite) OnTimeout(c *testing.T) {
	s.d.DumpStackAndQuit()
}

func (s *DockerRegistryAuthTokenSuite) SetUpTest(c *testing.T) {
	testRequires(c, DaemonIsLinux, RegistryHosting, testEnv.IsLocalDaemon)
	s.d = daemon.New(c, dockerBinary, dockerdBinary, testdaemon.WithEnvironment(testEnv.Execution))
}

func (s *DockerRegistryAuthTokenSuite) TearDownTest(c *testing.T) {
	if s.reg != nil {
		out, err := s.d.Cmd("logout", privateRegistryURL)
		assert.NilError(c, err, out)
		s.reg.Close()
	}
	if s.d != nil {
		s.d.Stop(c)
	}
	s.ds.TearDownTest(c)
}

func (s *DockerRegistryAuthTokenSuite) setupRegistryWithTokenService(c *testing.T, tokenURL string) {
	if s == nil {
		c.Fatal("registry suite isn't initialized")
	}
	s.reg = registry.NewV2(c, registry.Token(tokenURL))
	s.reg.WaitReady(c)
}

type DockerDaemonSuite struct {
	ds *DockerSuite
	d  *daemon.Daemon
}

func (s *DockerDaemonSuite) OnTimeout(c *testing.T) {
	s.d.DumpStackAndQuit()
}

func (s *DockerDaemonSuite) SetUpTest(c *testing.T) {
	testRequires(c, DaemonIsLinux, testEnv.IsLocalDaemon)
	s.d = daemon.New(c, dockerBinary, dockerdBinary, testdaemon.WithEnvironment(testEnv.Execution))
}

func (s *DockerDaemonSuite) TearDownTest(c *testing.T) {
	testRequires(c, DaemonIsLinux, testEnv.IsLocalDaemon)
	if s.d != nil {
		s.d.Stop(c)
	}
	s.ds.TearDownTest(c)
}

func (s *DockerDaemonSuite) TearDownSuite(c *testing.T) {
	filepath.Walk(testdaemon.SockRoot, func(path string, fi os.FileInfo, err error) error {
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
	os.RemoveAll(testdaemon.SockRoot)
}

const defaultSwarmPort = 2477

type DockerSwarmSuite struct {
	server      *httptest.Server
	ds          *DockerSuite
	daemonsLock sync.Mutex // protect access to daemons and portIndex
	daemons     []*daemon.Daemon
	portIndex   int
}

func (s *DockerSwarmSuite) OnTimeout(c *testing.T) {
	s.daemonsLock.Lock()
	defer s.daemonsLock.Unlock()
	for _, d := range s.daemons {
		d.DumpStackAndQuit()
	}
}

func (s *DockerSwarmSuite) SetUpTest(c *testing.T) {
	testRequires(c, DaemonIsLinux, testEnv.IsLocalDaemon)
}

func (s *DockerSwarmSuite) AddDaemon(c *testing.T, joinSwarm, manager bool) *daemon.Daemon {
	c.Helper()
	d := daemon.New(c, dockerBinary, dockerdBinary,
		testdaemon.WithEnvironment(testEnv.Execution),
		testdaemon.WithSwarmPort(defaultSwarmPort+s.portIndex),
	)
	if joinSwarm {
		if len(s.daemons) > 0 {
			d.StartAndSwarmJoin(c, s.daemons[0].Daemon, manager)
		} else {
			d.StartAndSwarmInit(c)
		}
	} else {
		d.StartNodeWithBusybox(c)
	}

	s.daemonsLock.Lock()
	s.portIndex++
	s.daemons = append(s.daemons, d)
	s.daemonsLock.Unlock()

	return d
}

func (s *DockerSwarmSuite) TearDownTest(c *testing.T) {
	testRequires(c, DaemonIsLinux)
	s.daemonsLock.Lock()
	for _, d := range s.daemons {
		if d != nil {
			d.Stop(c)
			d.Cleanup(c)
		}
	}
	s.daemons = nil
	s.portIndex = 0
	s.daemonsLock.Unlock()
	s.ds.TearDownTest(c)
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

func (ps *DockerPluginSuite) SetUpSuite(c *testing.T) {
	testRequires(c, DaemonIsLinux, RegistryHosting)
	ps.registry = registry.NewV2(c)
	ps.registry.WaitReady(c)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	err := plugin.CreateInRegistry(ctx, ps.getPluginRepo(), nil)
	assert.NilError(c, err, "failed to create plugin")
}

func (ps *DockerPluginSuite) TearDownSuite(c *testing.T) {
	if ps.registry != nil {
		ps.registry.Close()
	}
}

func (ps *DockerPluginSuite) TearDownTest(c *testing.T) {
	ps.ds.TearDownTest(c)
}

func (ps *DockerPluginSuite) OnTimeout(c *testing.T) {
	ps.ds.OnTimeout(c)
}
