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

	"github.com/moby/moby/v2/integration-cli/cli"
	"github.com/moby/moby/v2/integration-cli/daemon"
	"github.com/moby/moby/v2/integration-cli/environment"
	"github.com/moby/moby/v2/internal/test/suite"
	"github.com/moby/moby/v2/testutil"
	testdaemon "github.com/moby/moby/v2/testutil/daemon"
	ienv "github.com/moby/moby/v2/testutil/environment"
	"github.com/moby/moby/v2/testutil/fakestorage"
	"github.com/moby/moby/v2/testutil/fixtures/plugin"
	"github.com/moby/moby/v2/testutil/registry"
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

func testRun(m *testing.M) (exitCode int) {
	var retErr error

	shutdown := testutil.ConfigureTracing()
	ctx, span := otel.Tracer("").Start(context.Background(), "integration-cli/TestMain")
	defer func() {
		if retErr != nil {
			span.SetStatus(codes.Error, retErr.Error())
			if exitCode == 0 {
				// Should never happen, but in case we forgot to set a code :)
				exitCode = 255
			}
		}
		if exitCode != 0 {
			span.SetAttributes(attribute.Int("exitCode", exitCode))
			span.SetStatus(codes.Error, "m.Run() exited with non-zero code")
		}
		span.End()
		shutdown(ctx)
	}()

	baseContext = ctx

	testEnv, retErr = environment.New(ctx)
	if retErr != nil {
		return 255
	}

	if testEnv.IsLocalDaemon() {
		setupLocalInfo()
	}

	dockerBinary = testEnv.DockerBinary()

	retErr = ienv.EnsureFrozenImagesLinux(ctx, &testEnv.Execution)
	if retErr != nil {
		return 255
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

func (s *DockerSuite) OnTimeout(t *testing.T) {
	if testEnv.IsRemoteDaemon() {
		return
	}
	pidFile := filepath.Join(os.Getenv("DEST"), "docker.pid")
	b, err := os.ReadFile(pidFile)
	if err != nil {
		t.Fatalf("Failed to get daemon PID from %s\n", pidFile)
	}

	rawPid, err := strconv.ParseInt(string(b), 10, 32)
	if err != nil {
		t.Fatalf("Failed to parse pid from %s: %s\n", pidFile, err)
	}

	if daemonPid := int(rawPid); daemonPid > 0 {
		testdaemon.SignalDaemonDump(daemonPid)
	}
}

func (s *DockerSuite) TearDownTest(ctx context.Context, t *testing.T) {
	testEnv.Clean(ctx, t)
}

type DockerRegistrySuite struct {
	ds  *DockerSuite
	reg *registry.V2
	d   *daemon.Daemon
}

func (s *DockerRegistrySuite) OnTimeout(t *testing.T) {
	s.d.DumpStackAndQuit()
}

func (s *DockerRegistrySuite) SetUpTest(ctx context.Context, t *testing.T) {
	testRequires(t, DaemonIsLinux, RegistryHosting, testEnv.IsLocalDaemon)
	s.reg = registry.NewV2(t)
	s.reg.WaitReady(t)
	s.d = daemon.New(t, dockerBinary, dockerdBinary, testdaemon.WithEnvironment(testEnv.Execution))
}

func (s *DockerRegistrySuite) TearDownTest(ctx context.Context, t *testing.T) {
	if s.reg != nil {
		s.reg.Close()
	}
	if s.d != nil {
		s.d.Stop(t)
	}
	s.ds.TearDownTest(ctx, t)
}

type DockerRegistryAuthHtpasswdSuite struct {
	ds  *DockerSuite
	reg *registry.V2
	d   *daemon.Daemon
}

func (s *DockerRegistryAuthHtpasswdSuite) OnTimeout(t *testing.T) {
	s.d.DumpStackAndQuit()
}

func (s *DockerRegistryAuthHtpasswdSuite) SetUpTest(ctx context.Context, t *testing.T) {
	testRequires(t, DaemonIsLinux, RegistryHosting, testEnv.IsLocalDaemon)
	s.reg = registry.NewV2(t, registry.Htpasswd)
	s.reg.WaitReady(t)
	s.d = daemon.New(t, dockerBinary, dockerdBinary, testdaemon.WithEnvironment(testEnv.Execution))
}

func (s *DockerRegistryAuthHtpasswdSuite) TearDownTest(ctx context.Context, t *testing.T) {
	if s.reg != nil {
		out, err := s.d.Cmd("logout", privateRegistryURL)
		assert.NilError(t, err, out)
		s.reg.Close()
	}
	if s.d != nil {
		s.d.Stop(t)
	}
	s.ds.TearDownTest(ctx, t)
}

type DockerRegistryAuthTokenSuite struct {
	ds  *DockerSuite
	reg *registry.V2
	d   *daemon.Daemon
}

func (s *DockerRegistryAuthTokenSuite) OnTimeout(t *testing.T) {
	s.d.DumpStackAndQuit()
}

func (s *DockerRegistryAuthTokenSuite) SetUpTest(ctx context.Context, t *testing.T) {
	testRequires(t, DaemonIsLinux, RegistryHosting, testEnv.IsLocalDaemon)
	s.d = daemon.New(t, dockerBinary, dockerdBinary, testdaemon.WithEnvironment(testEnv.Execution))
}

func (s *DockerRegistryAuthTokenSuite) TearDownTest(ctx context.Context, t *testing.T) {
	if s.reg != nil {
		out, err := s.d.Cmd("logout", privateRegistryURL)
		assert.NilError(t, err, out)
		s.reg.Close()
	}
	if s.d != nil {
		s.d.Stop(t)
	}
	s.ds.TearDownTest(ctx, t)
}

func (s *DockerRegistryAuthTokenSuite) setupRegistryWithTokenService(t *testing.T, tokenURL string) {
	if s == nil {
		t.Fatal("registry suite isn't initialized")
	}
	s.reg = registry.NewV2(t, registry.Token(tokenURL))
	s.reg.WaitReady(t)
}

type DockerDaemonSuite struct {
	ds *DockerSuite
	d  *daemon.Daemon
}

func (s *DockerDaemonSuite) OnTimeout(t *testing.T) {
	s.d.DumpStackAndQuit()
}

func (s *DockerDaemonSuite) SetUpTest(ctx context.Context, t *testing.T) {
	testRequires(t, DaemonIsLinux, testEnv.IsLocalDaemon)
	s.d = daemon.New(t, dockerBinary, dockerdBinary, testdaemon.WithEnvironment(testEnv.Execution))
}

func (s *DockerDaemonSuite) TearDownTest(ctx context.Context, t *testing.T) {
	testRequires(t, DaemonIsLinux, testEnv.IsLocalDaemon)
	if s.d != nil {
		s.d.Stop(t)
	}
	s.ds.TearDownTest(ctx, t)
}

func (s *DockerDaemonSuite) TearDownSuite(ctx context.Context, t *testing.T) {
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

func (s *DockerSwarmSuite) OnTimeout(t *testing.T) {
	s.daemonsLock.Lock()
	defer s.daemonsLock.Unlock()
	for _, d := range s.daemons {
		d.DumpStackAndQuit()
	}
}

func (s *DockerSwarmSuite) SetUpTest(ctx context.Context, t *testing.T) {
	testRequires(t, DaemonIsLinux, testEnv.IsLocalDaemon)
}

func (s *DockerSwarmSuite) AddDaemon(ctx context.Context, t *testing.T, joinSwarm, manager bool) *daemon.Daemon {
	t.Helper()
	d := daemon.New(t, dockerBinary, dockerdBinary,
		testdaemon.WithEnvironment(testEnv.Execution),
		testdaemon.WithSwarmPort(defaultSwarmPort+s.portIndex),
	)
	if joinSwarm {
		if len(s.daemons) > 0 {
			d.StartAndSwarmJoin(ctx, t, s.daemons[0].Daemon, manager)
		} else {
			d.StartAndSwarmInit(ctx, t)
		}
	} else {
		d.StartNodeWithBusybox(ctx, t)
	}

	s.daemonsLock.Lock()
	s.portIndex++
	s.daemons = append(s.daemons, d)
	s.daemonsLock.Unlock()

	return d
}

func (s *DockerSwarmSuite) TearDownTest(ctx context.Context, t *testing.T) {
	testRequires(t, DaemonIsLinux)
	s.daemonsLock.Lock()
	for _, d := range s.daemons {
		if d != nil {
			if t.Failed() {
				d.TailLogsT(t, 100)
			}
			d.Stop(t)
			d.Cleanup(t)
		}
	}
	s.daemons = nil
	s.portIndex = 0
	s.daemonsLock.Unlock()
	s.ds.TearDownTest(ctx, t)
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

func (ps *DockerPluginSuite) SetUpSuite(ctx context.Context, t *testing.T) {
	testRequires(t, DaemonIsLinux, RegistryHosting)
	ps.registry = registry.NewV2(t)
	ps.registry.WaitReady(t)

	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	err := plugin.CreateInRegistry(ctx, ps.getPluginRepo(), nil)
	assert.NilError(t, err, "failed to create plugin")
}

func (ps *DockerPluginSuite) TearDownSuite(ctx context.Context, t *testing.T) {
	if ps.registry != nil {
		ps.registry.Close()
	}
}

func (ps *DockerPluginSuite) TearDownTest(ctx context.Context, t *testing.T) {
	ps.ds.TearDownTest(ctx, t)
}

func (ps *DockerPluginSuite) OnTimeout(t *testing.T) {
	ps.ds.OnTimeout(t)
}
