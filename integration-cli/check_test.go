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
	"github.com/docker/docker/testutil"
	testdaemon "github.com/docker/docker/testutil/daemon"
	ienv "github.com/docker/docker/testutil/environment"
	"github.com/docker/docker/testutil/fakestorage"
	"github.com/docker/docker/testutil/fixtures/plugin"
	"github.com/docker/docker/testutil/registry"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
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
	testEnvOnce sync.Once
	testEnv     *environment.Execution

	// the docker client binary to use
	dockerBinary = ""

	baseContext context.Context
)

func TestMain(m *testing.M) {
	flag.Parse()

	os.Exit(testRun(m))
}

func testRun(m *testing.M) (ret int) {
	// Global set up

	var err error

	shutdown := testutil.ConfigureTracing()
	ctx, span := otel.Tracer("").Start(context.Background(), "integration-cli/TestMain")
	defer func() {
		if err != nil {
			span.SetStatus(codes.Error, err.Error())
			ret = 255
		} else {
			if ret != 0 {
				span.SetAttributes(attribute.Int("exitCode", ret))
				span.SetStatus(codes.Error, "m.Run() exited with non-zero code")
			}
		}
		span.End()
		shutdown(ctx)
	}()

	baseContext = ctx

	testEnv, err = environment.New(ctx)
	if err != nil {
		return
	}

	if testEnv.IsLocalDaemon() {
		setupLocalInfo()
	}

	dockerBinary = testEnv.DockerBinary()

	err = ienv.EnsureFrozenImagesLinux(ctx, &testEnv.Execution)
	if err != nil {
		return
	}

	testEnv.Print()
	printCliVersion()

	return m.Run()
}

func printCliVersion() {
	// Print output of "docker version"
	cli.SetTestEnvironment(testEnv)
	cmd := cli.Docker(cli.Args("version"))
	if cmd.Error != nil {
		fmt.Printf("WARNING: Failed to run 'docker version': %+v\n", cmd.Error)
		return
	}

	fmt.Println("INFO: Testing with docker cli version:")
	fmt.Println(cmd.Stdout())
}

func ensureTestEnvSetup(ctx context.Context, t *testing.T) {
	testEnvOnce.Do(func() {
		cli.SetTestEnvironment(testEnv)
		fakestorage.SetTestEnvironment(&testEnv.Execution)
		ienv.ProtectAll(ctx, t, &testEnv.Execution)
	})
}

func TestDockerAPISuite(t *testing.T) {
	ctx := testutil.StartSpan(baseContext, t)
	ensureTestEnvSetup(ctx, t)
	suite.Run(ctx, t, &DockerAPISuite{ds: &DockerSuite{}})
}

func TestDockerBenchmarkSuite(t *testing.T) {
	ctx := testutil.StartSpan(baseContext, t)
	ensureTestEnvSetup(ctx, t)
	suite.Run(ctx, t, &DockerBenchmarkSuite{ds: &DockerSuite{}})
}

func TestDockerCLIAttachSuite(t *testing.T) {
	ctx := testutil.StartSpan(baseContext, t)
	ensureTestEnvSetup(ctx, t)
	suite.Run(ctx, t, &DockerCLIAttachSuite{ds: &DockerSuite{}})
}

func TestDockerCLIBuildSuite(t *testing.T) {
	ctx := testutil.StartSpan(baseContext, t)
	ensureTestEnvSetup(ctx, t)
	suite.Run(ctx, t, &DockerCLIBuildSuite{ds: &DockerSuite{}})
}

func TestDockerCLICommitSuite(t *testing.T) {
	ctx := testutil.StartSpan(baseContext, t)
	ensureTestEnvSetup(ctx, t)
	suite.Run(ctx, t, &DockerCLICommitSuite{ds: &DockerSuite{}})
}

func TestDockerCLICpSuite(t *testing.T) {
	ctx := testutil.StartSpan(baseContext, t)
	ensureTestEnvSetup(ctx, t)
	suite.Run(ctx, t, &DockerCLICpSuite{ds: &DockerSuite{}})
}

func TestDockerCLICreateSuite(t *testing.T) {
	ctx := testutil.StartSpan(baseContext, t)
	ensureTestEnvSetup(ctx, t)
	suite.Run(ctx, t, &DockerCLICreateSuite{ds: &DockerSuite{}})
}

func TestDockerCLIEventSuite(t *testing.T) {
	ctx := testutil.StartSpan(baseContext, t)
	ensureTestEnvSetup(ctx, t)
	suite.Run(ctx, t, &DockerCLIEventSuite{ds: &DockerSuite{}})
}

func TestDockerCLIExecSuite(t *testing.T) {
	ctx := testutil.StartSpan(baseContext, t)
	ensureTestEnvSetup(ctx, t)
	suite.Run(ctx, t, &DockerCLIExecSuite{ds: &DockerSuite{}})
}

func TestDockerCLIHealthSuite(t *testing.T) {
	ctx := testutil.StartSpan(baseContext, t)
	ensureTestEnvSetup(ctx, t)
	suite.Run(ctx, t, &DockerCLIHealthSuite{ds: &DockerSuite{}})
}

func TestDockerCLIHistorySuite(t *testing.T) {
	ctx := testutil.StartSpan(baseContext, t)
	ensureTestEnvSetup(ctx, t)
	suite.Run(ctx, t, &DockerCLIHistorySuite{ds: &DockerSuite{}})
}

func TestDockerCLIImagesSuite(t *testing.T) {
	ctx := testutil.StartSpan(baseContext, t)
	ensureTestEnvSetup(ctx, t)
	suite.Run(ctx, t, &DockerCLIImagesSuite{ds: &DockerSuite{}})
}

func TestDockerCLIImportSuite(t *testing.T) {
	ctx := testutil.StartSpan(baseContext, t)
	ensureTestEnvSetup(ctx, t)
	suite.Run(ctx, t, &DockerCLIImportSuite{ds: &DockerSuite{}})
}

func TestDockerCLIInfoSuite(t *testing.T) {
	ctx := testutil.StartSpan(baseContext, t)
	ensureTestEnvSetup(ctx, t)
	suite.Run(ctx, t, &DockerCLIInfoSuite{ds: &DockerSuite{}})
}

func TestDockerCLIInspectSuite(t *testing.T) {
	ctx := testutil.StartSpan(baseContext, t)
	ensureTestEnvSetup(ctx, t)
	suite.Run(ctx, t, &DockerCLIInspectSuite{ds: &DockerSuite{}})
}

func TestDockerCLILinksSuite(t *testing.T) {
	ctx := testutil.StartSpan(baseContext, t)
	ensureTestEnvSetup(ctx, t)
	suite.Run(ctx, t, &DockerCLILinksSuite{ds: &DockerSuite{}})
}

func TestDockerCLILoginSuite(t *testing.T) {
	ctx := testutil.StartSpan(baseContext, t)
	ensureTestEnvSetup(ctx, t)
	suite.Run(ctx, t, &DockerCLILoginSuite{ds: &DockerSuite{}})
}

func TestDockerCLILogsSuite(t *testing.T) {
	ctx := testutil.StartSpan(baseContext, t)
	ensureTestEnvSetup(ctx, t)
	suite.Run(ctx, t, &DockerCLILogsSuite{ds: &DockerSuite{}})
}

func TestDockerCLINetmodeSuite(t *testing.T) {
	ctx := testutil.StartSpan(baseContext, t)
	ensureTestEnvSetup(ctx, t)
	suite.Run(ctx, t, &DockerCLINetmodeSuite{ds: &DockerSuite{}})
}

func TestDockerCLINetworkSuite(t *testing.T) {
	ctx := testutil.StartSpan(baseContext, t)
	ensureTestEnvSetup(ctx, t)
	suite.Run(ctx, t, &DockerCLINetworkSuite{ds: &DockerSuite{}})
}

func TestDockerCLIPluginLogDriverSuite(t *testing.T) {
	ctx := testutil.StartSpan(baseContext, t)
	ensureTestEnvSetup(ctx, t)
	suite.Run(ctx, t, &DockerCLIPluginLogDriverSuite{ds: &DockerSuite{}})
}

func TestDockerCLIPluginsSuite(t *testing.T) {
	ctx := testutil.StartSpan(baseContext, t)
	ensureTestEnvSetup(ctx, t)
	suite.Run(ctx, t, &DockerCLIPluginsSuite{ds: &DockerSuite{}})
}

func TestDockerCLIPortSuite(t *testing.T) {
	ctx := testutil.StartSpan(baseContext, t)
	ensureTestEnvSetup(ctx, t)
	suite.Run(ctx, t, &DockerCLIPortSuite{ds: &DockerSuite{}})
}

func TestDockerCLIProxySuite(t *testing.T) {
	ctx := testutil.StartSpan(baseContext, t)
	ensureTestEnvSetup(ctx, t)
	suite.Run(ctx, t, &DockerCLIProxySuite{ds: &DockerSuite{}})
}

func TestDockerCLIPruneSuite(t *testing.T) {
	ctx := testutil.StartSpan(baseContext, t)
	ensureTestEnvSetup(ctx, t)
	suite.Run(ctx, t, &DockerCLIPruneSuite{ds: &DockerSuite{}})
}

func TestDockerCLIPsSuite(t *testing.T) {
	ctx := testutil.StartSpan(baseContext, t)
	ensureTestEnvSetup(ctx, t)
	suite.Run(ctx, t, &DockerCLIPsSuite{ds: &DockerSuite{}})
}

func TestDockerCLIPullSuite(t *testing.T) {
	ctx := testutil.StartSpan(baseContext, t)
	ensureTestEnvSetup(ctx, t)
	suite.Run(ctx, t, &DockerCLIPullSuite{ds: &DockerSuite{}})
}

func TestDockerCLIPushSuite(t *testing.T) {
	ctx := testutil.StartSpan(baseContext, t)
	ensureTestEnvSetup(ctx, t)
	suite.Run(ctx, t, &DockerCLIPushSuite{ds: &DockerSuite{}})
}

func TestDockerCLIRestartSuite(t *testing.T) {
	ctx := testutil.StartSpan(baseContext, t)
	ensureTestEnvSetup(ctx, t)
	suite.Run(ctx, t, &DockerCLIRestartSuite{ds: &DockerSuite{}})
}

func TestDockerCLIRmiSuite(t *testing.T) {
	ctx := testutil.StartSpan(baseContext, t)
	ensureTestEnvSetup(ctx, t)
	suite.Run(ctx, t, &DockerCLIRmiSuite{ds: &DockerSuite{}})
}

func TestDockerCLIRunSuite(t *testing.T) {
	ctx := testutil.StartSpan(baseContext, t)
	ensureTestEnvSetup(ctx, t)
	suite.Run(ctx, t, &DockerCLIRunSuite{ds: &DockerSuite{}})
}

func TestDockerCLISaveLoadSuite(t *testing.T) {
	ctx := testutil.StartSpan(baseContext, t)
	ensureTestEnvSetup(ctx, t)
	suite.Run(ctx, t, &DockerCLISaveLoadSuite{ds: &DockerSuite{}})
}

func TestDockerCLISearchSuite(t *testing.T) {
	ctx := testutil.StartSpan(baseContext, t)
	ensureTestEnvSetup(ctx, t)
	suite.Run(ctx, t, &DockerCLISearchSuite{ds: &DockerSuite{}})
}

func TestDockerCLISNISuite(t *testing.T) {
	ctx := testutil.StartSpan(baseContext, t)
	ensureTestEnvSetup(ctx, t)
	suite.Run(ctx, t, &DockerCLISNISuite{ds: &DockerSuite{}})
}

func TestDockerCLIStartSuite(t *testing.T) {
	ctx := testutil.StartSpan(baseContext, t)
	ensureTestEnvSetup(ctx, t)
	suite.Run(ctx, t, &DockerCLIStartSuite{ds: &DockerSuite{}})
}

func TestDockerCLIStatsSuite(t *testing.T) {
	ctx := testutil.StartSpan(baseContext, t)
	ensureTestEnvSetup(ctx, t)
	suite.Run(ctx, t, &DockerCLIStatsSuite{ds: &DockerSuite{}})
}

func TestDockerCLITopSuite(t *testing.T) {
	ctx := testutil.StartSpan(baseContext, t)
	ensureTestEnvSetup(ctx, t)
	suite.Run(ctx, t, &DockerCLITopSuite{ds: &DockerSuite{}})
}

func TestDockerCLIUpdateSuite(t *testing.T) {
	ctx := testutil.StartSpan(baseContext, t)
	ensureTestEnvSetup(ctx, t)
	suite.Run(ctx, t, &DockerCLIUpdateSuite{ds: &DockerSuite{}})
}

func TestDockerCLIVolumeSuite(t *testing.T) {
	ctx := testutil.StartSpan(baseContext, t)
	ensureTestEnvSetup(ctx, t)
	suite.Run(ctx, t, &DockerCLIVolumeSuite{ds: &DockerSuite{}})
}

func TestDockerRegistrySuite(t *testing.T) {
	ctx := testutil.StartSpan(baseContext, t)
	ensureTestEnvSetup(ctx, t)
	suite.Run(ctx, t, &DockerRegistrySuite{ds: &DockerSuite{}})
}

func TestDockerSchema1RegistrySuite(t *testing.T) {
	ctx := testutil.StartSpan(baseContext, t)
	ensureTestEnvSetup(ctx, t)
	suite.Run(ctx, t, &DockerSchema1RegistrySuite{ds: &DockerSuite{}})
}

func TestDockerRegistryAuthHtpasswdSuite(t *testing.T) {
	ctx := testutil.StartSpan(baseContext, t)
	ensureTestEnvSetup(ctx, t)
	suite.Run(ctx, t, &DockerRegistryAuthHtpasswdSuite{ds: &DockerSuite{}})
}

func TestDockerRegistryAuthTokenSuite(t *testing.T) {
	ctx := testutil.StartSpan(baseContext, t)
	ensureTestEnvSetup(ctx, t)
	suite.Run(ctx, t, &DockerRegistryAuthTokenSuite{ds: &DockerSuite{}})
}

func TestDockerDaemonSuite(t *testing.T) {
	ctx := testutil.StartSpan(baseContext, t)
	ensureTestEnvSetup(ctx, t)
	suite.Run(ctx, t, &DockerDaemonSuite{ds: &DockerSuite{}})
}

func TestDockerSwarmSuite(t *testing.T) {
	ctx := testutil.StartSpan(baseContext, t)
	ensureTestEnvSetup(ctx, t)
	suite.Run(ctx, t, &DockerSwarmSuite{ds: &DockerSuite{}})
}

func TestDockerPluginSuite(t *testing.T) {
	ctx := testutil.StartSpan(baseContext, t)
	ensureTestEnvSetup(ctx, t)
	suite.Run(ctx, t, &DockerPluginSuite{ds: &DockerSuite{}})
}

func TestDockerExternalVolumeSuite(t *testing.T) {
	testRequires(t, DaemonIsLinux)
	ctx := testutil.StartSpan(baseContext, t)
	ensureTestEnvSetup(ctx, t)
	suite.Run(ctx, t, &DockerExternalVolumeSuite{ds: &DockerSuite{}})
}

func TestDockerNetworkSuite(t *testing.T) {
	testRequires(t, DaemonIsLinux)
	ctx := testutil.StartSpan(baseContext, t)
	ensureTestEnvSetup(ctx, t)
	suite.Run(ctx, t, &DockerNetworkSuite{ds: &DockerSuite{}})
}

func TestDockerHubPullSuite(t *testing.T) {
	ctx := testutil.StartSpan(baseContext, t)
	ensureTestEnvSetup(ctx, t)
	// FIXME. Temporarily turning this off for Windows as GH16039 was breaking
	// Windows to Linux CI @icecrime
	testRequires(t, DaemonIsLinux)
	suite.Run(ctx, t, newDockerHubPullSuite())
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

func (s *DockerSuite) TearDownTest(ctx context.Context, c *testing.T) {
	testEnv.Clean(ctx, c)
}

type DockerRegistrySuite struct {
	ds  *DockerSuite
	reg *registry.V2
	d   *daemon.Daemon
}

func (s *DockerRegistrySuite) OnTimeout(c *testing.T) {
	s.d.DumpStackAndQuit()
}

func (s *DockerRegistrySuite) SetUpTest(ctx context.Context, c *testing.T) {
	testRequires(c, DaemonIsLinux, RegistryHosting, testEnv.IsLocalDaemon)
	s.reg = registry.NewV2(c)
	s.reg.WaitReady(c)
	s.d = daemon.New(c, dockerBinary, dockerdBinary, testdaemon.WithEnvironment(testEnv.Execution))
}

func (s *DockerRegistrySuite) TearDownTest(ctx context.Context, c *testing.T) {
	if s.reg != nil {
		s.reg.Close()
	}
	if s.d != nil {
		s.d.Stop(c)
	}
	s.ds.TearDownTest(ctx, c)
}

type DockerSchema1RegistrySuite struct {
	ds  *DockerSuite
	reg *registry.V2
	d   *daemon.Daemon
}

func (s *DockerSchema1RegistrySuite) OnTimeout(c *testing.T) {
	s.d.DumpStackAndQuit()
}

func (s *DockerSchema1RegistrySuite) SetUpTest(ctx context.Context, c *testing.T) {
	testRequires(c, DaemonIsLinux, RegistryHosting, NotArm64, testEnv.IsLocalDaemon)
	s.reg = registry.NewV2(c, registry.Schema1)
	s.reg.WaitReady(c)
	s.d = daemon.New(c, dockerBinary, dockerdBinary, testdaemon.WithEnvironment(testEnv.Execution))
}

func (s *DockerSchema1RegistrySuite) TearDownTest(ctx context.Context, c *testing.T) {
	if s.reg != nil {
		s.reg.Close()
	}
	if s.d != nil {
		s.d.Stop(c)
	}
	s.ds.TearDownTest(ctx, c)
}

type DockerRegistryAuthHtpasswdSuite struct {
	ds  *DockerSuite
	reg *registry.V2
	d   *daemon.Daemon
}

func (s *DockerRegistryAuthHtpasswdSuite) OnTimeout(c *testing.T) {
	s.d.DumpStackAndQuit()
}

func (s *DockerRegistryAuthHtpasswdSuite) SetUpTest(ctx context.Context, c *testing.T) {
	testRequires(c, DaemonIsLinux, RegistryHosting, testEnv.IsLocalDaemon)
	s.reg = registry.NewV2(c, registry.Htpasswd)
	s.reg.WaitReady(c)
	s.d = daemon.New(c, dockerBinary, dockerdBinary, testdaemon.WithEnvironment(testEnv.Execution))
}

func (s *DockerRegistryAuthHtpasswdSuite) TearDownTest(ctx context.Context, c *testing.T) {
	if s.reg != nil {
		out, err := s.d.Cmd("logout", privateRegistryURL)
		assert.NilError(c, err, out)
		s.reg.Close()
	}
	if s.d != nil {
		s.d.Stop(c)
	}
	s.ds.TearDownTest(ctx, c)
}

type DockerRegistryAuthTokenSuite struct {
	ds  *DockerSuite
	reg *registry.V2
	d   *daemon.Daemon
}

func (s *DockerRegistryAuthTokenSuite) OnTimeout(c *testing.T) {
	s.d.DumpStackAndQuit()
}

func (s *DockerRegistryAuthTokenSuite) SetUpTest(ctx context.Context, c *testing.T) {
	testRequires(c, DaemonIsLinux, RegistryHosting, testEnv.IsLocalDaemon)
	s.d = daemon.New(c, dockerBinary, dockerdBinary, testdaemon.WithEnvironment(testEnv.Execution))
}

func (s *DockerRegistryAuthTokenSuite) TearDownTest(ctx context.Context, c *testing.T) {
	if s.reg != nil {
		out, err := s.d.Cmd("logout", privateRegistryURL)
		assert.NilError(c, err, out)
		s.reg.Close()
	}
	if s.d != nil {
		s.d.Stop(c)
	}
	s.ds.TearDownTest(ctx, c)
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

func (s *DockerDaemonSuite) SetUpTest(ctx context.Context, c *testing.T) {
	testRequires(c, DaemonIsLinux, testEnv.IsLocalDaemon)
	s.d = daemon.New(c, dockerBinary, dockerdBinary, testdaemon.WithEnvironment(testEnv.Execution))
}

func (s *DockerDaemonSuite) TearDownTest(ctx context.Context, c *testing.T) {
	testRequires(c, DaemonIsLinux, testEnv.IsLocalDaemon)
	if s.d != nil {
		s.d.Stop(c)
	}
	s.ds.TearDownTest(ctx, c)
}

func (s *DockerDaemonSuite) TearDownSuite(ctx context.Context, c *testing.T) {
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

func (s *DockerSwarmSuite) SetUpTest(ctx context.Context, c *testing.T) {
	testRequires(c, DaemonIsLinux, testEnv.IsLocalDaemon)
}

func (s *DockerSwarmSuite) AddDaemon(ctx context.Context, c *testing.T, joinSwarm, manager bool) *daemon.Daemon {
	c.Helper()
	d := daemon.New(c, dockerBinary, dockerdBinary,
		testdaemon.WithEnvironment(testEnv.Execution),
		testdaemon.WithSwarmPort(defaultSwarmPort+s.portIndex),
	)
	if joinSwarm {
		if len(s.daemons) > 0 {
			d.StartAndSwarmJoin(ctx, c, s.daemons[0].Daemon, manager)
		} else {
			d.StartAndSwarmInit(ctx, c)
		}
	} else {
		d.StartNodeWithBusybox(ctx, c)
	}

	s.daemonsLock.Lock()
	s.portIndex++
	s.daemons = append(s.daemons, d)
	s.daemonsLock.Unlock()

	return d
}

func (s *DockerSwarmSuite) TearDownTest(ctx context.Context, c *testing.T) {
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
	s.ds.TearDownTest(ctx, c)
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

func (ps *DockerPluginSuite) SetUpSuite(ctx context.Context, c *testing.T) {
	testRequires(c, DaemonIsLinux, RegistryHosting)
	ps.registry = registry.NewV2(c)
	ps.registry.WaitReady(c)

	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	err := plugin.CreateInRegistry(ctx, ps.getPluginRepo(), nil)
	assert.NilError(c, err, "failed to create plugin")
}

func (ps *DockerPluginSuite) TearDownSuite(ctx context.Context, c *testing.T) {
	if ps.registry != nil {
		ps.registry.Close()
	}
}

func (ps *DockerPluginSuite) TearDownTest(ctx context.Context, c *testing.T) {
	ps.ds.TearDownTest(ctx, c)
}

func (ps *DockerPluginSuite) OnTimeout(c *testing.T) {
	ps.ds.OnTimeout(c)
}
