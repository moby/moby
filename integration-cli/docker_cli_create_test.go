package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/moby/moby/api/types/network"
	"github.com/moby/moby/v2/integration-cli/cli"
	"github.com/moby/moby/v2/integration-cli/cli/build"
	"github.com/moby/moby/v2/internal/testutil/fakecontext"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

type DockerCLICreateSuite struct {
	ds *DockerSuite
}

func (s *DockerCLICreateSuite) TearDownTest(ctx context.Context, t *testing.T) {
	s.ds.TearDownTest(ctx, t)
}

func (s *DockerCLICreateSuite) OnTimeout(t *testing.T) {
	s.ds.OnTimeout(t)
}

// Make sure we can create a simple container with some args
func (s *DockerCLICreateSuite) TestCreateArgs(c *testing.T) {
	// Intentionally clear entrypoint, as the Windows busybox image needs an entrypoint, which breaks this test
	containerID := cli.DockerCmd(c, "create", "--entrypoint=", "busybox", "command", "arg1", "arg2", "arg with space", "-c", "flags").Stdout()
	containerID = strings.TrimSpace(containerID)

	out := cli.DockerCmd(c, "inspect", containerID).Combined()

	var containers []struct {
		Path string
		Args []string
	}

	err := json.Unmarshal([]byte(out), &containers)
	assert.Assert(c, err == nil, "Error inspecting the container: %s", err)
	assert.Equal(c, len(containers), 1)

	cont := containers[0]
	assert.Equal(c, cont.Path, "command", fmt.Sprintf("Unexpected container path. Expected command, received: %s", cont.Path))

	b := false
	expected := []string{"arg1", "arg2", "arg with space", "-c", "flags"}
	for i, arg := range expected {
		if arg != cont.Args[i] {
			b = true
			break
		}
	}
	if len(cont.Args) != len(expected) || b {
		c.Fatalf("Unexpected args. Expected %v, received: %v", expected, cont.Args)
	}
}

// Make sure we can set hostconfig options too
func (s *DockerCLICreateSuite) TestCreateHostConfig(c *testing.T) {
	containerID := cli.DockerCmd(c, "create", "-P", "busybox", "echo").Stdout()
	containerID = strings.TrimSpace(containerID)

	out := cli.DockerCmd(c, "inspect", containerID).Stdout()

	var containers []struct {
		HostConfig *struct {
			PublishAllPorts bool
		}
	}

	err := json.Unmarshal([]byte(out), &containers)
	assert.Assert(c, err == nil, "Error inspecting the container: %s", err)
	assert.Equal(c, len(containers), 1)

	cont := containers[0]
	assert.Assert(c, cont.HostConfig != nil, "Expected HostConfig, got none")
	assert.Assert(c, cont.HostConfig.PublishAllPorts, "Expected PublishAllPorts, got false")
}

func (s *DockerCLICreateSuite) TestCreateWithPortRange(c *testing.T) {
	containerID := cli.DockerCmd(c, "create", "-p", "3300-3303:3300-3303/tcp", "busybox", "echo").Stdout()
	containerID = strings.TrimSpace(containerID)

	out := cli.DockerCmd(c, "inspect", containerID).Stdout()

	var containers []struct {
		HostConfig *struct {
			PortBindings network.PortMap
		}
	}
	err := json.Unmarshal([]byte(out), &containers)
	assert.Assert(c, err == nil, "Error inspecting the container: %s", err)
	assert.Equal(c, len(containers), 1)

	cont := containers[0]

	assert.Assert(c, cont.HostConfig != nil, "Expected HostConfig, got none")
	assert.Equal(c, len(cont.HostConfig.PortBindings), 4, fmt.Sprintf("Expected 4 ports bindings, got %d", len(cont.HostConfig.PortBindings)))

	for k, v := range cont.HostConfig.PortBindings {
		assert.Equal(c, len(v), 1, fmt.Sprintf("Expected 1 ports binding, for the port %s but found %s", k, v))
		assert.Equal(c, fmt.Sprintf("%d", k.Num()), v[0].HostPort, fmt.Sprintf("Expected host port %d to match published port %s", k.Num(), v[0].HostPort))
	}
}

func (s *DockerCLICreateSuite) TestCreateWithLargePortRange(c *testing.T) {
	containerID := cli.DockerCmd(c, "create", "-p", "1-65535:1-65535/tcp", "busybox", "echo").Stdout()
	containerID = strings.TrimSpace(containerID)

	out := cli.DockerCmd(c, "inspect", containerID).Stdout()

	var containers []struct {
		HostConfig *struct {
			PortBindings network.PortMap
		}
	}

	err := json.Unmarshal([]byte(out), &containers)
	assert.Assert(c, err == nil, "Error inspecting the container: %s", err)
	assert.Equal(c, len(containers), 1)

	cont := containers[0]
	assert.Assert(c, cont.HostConfig != nil, "Expected HostConfig, got none")
	assert.Equal(c, len(cont.HostConfig.PortBindings), 65535)

	for k, v := range cont.HostConfig.PortBindings {
		assert.Equal(c, len(v), 1)
		assert.Equal(c, fmt.Sprintf("%d", k.Num()), v[0].HostPort, fmt.Sprintf("Expected host port %d to match published port %s", k.Num(), v[0].HostPort))
	}
}

// "test123" should be printed by docker create + start
func (s *DockerCLICreateSuite) TestCreateEchoStdout(c *testing.T) {
	containerID := cli.DockerCmd(c, "create", "busybox", "echo", "test123").Stdout()
	containerID = strings.TrimSpace(containerID)

	out := cli.DockerCmd(c, "start", "-ai", containerID).Combined()
	assert.Equal(c, out, "test123\n", "container should've printed 'test123', got %q", out)
}

func (s *DockerCLICreateSuite) TestCreateVolumesCreated(c *testing.T) {
	testRequires(c, testEnv.IsLocalDaemon)
	prefix, slash := getPrefixAndSlashFromDaemonPlatform()

	const name = "test_create_volume"
	cli.DockerCmd(c, "create", "--name", name, "-v", prefix+slash+"foo", "busybox")

	mnt, err := inspectMountPoint(name, prefix+slash+"foo")
	assert.Assert(c, err == nil, "Error getting volume host path: %q", err)

	if _, err := os.Stat(mnt.Source); err != nil && os.IsNotExist(err) {
		c.Fatalf("Volume was not created")
	}
	if err != nil {
		c.Fatalf("Error statting volume host path: %q", err)
	}
}

func (s *DockerCLICreateSuite) TestCreateLabels(c *testing.T) {
	const name = "test_create_labels"
	expected := map[string]string{"k1": "v1", "k2": "v2"}
	cli.DockerCmd(c, "create", "--name", name, "-l", "k1=v1", "--label", "k2=v2", "busybox")

	actual := make(map[string]string)
	inspectFieldAndUnmarshall(c, name, "Config.Labels", &actual)

	if !reflect.DeepEqual(expected, actual) {
		c.Fatalf("Expected %s got %s", expected, actual)
	}
}

func (s *DockerCLICreateSuite) TestCreateLabelFromImage(c *testing.T) {
	imageName := "testcreatebuildlabel"
	cli.BuildCmd(c, imageName, build.WithDockerfile(`FROM busybox
		LABEL k1=v1 k2=v2`))

	const name = "test_create_labels_from_image"
	expected := map[string]string{"k2": "x", "k3": "v3", "k1": "v1"}
	cli.DockerCmd(c, "create", "--name", name, "-l", "k2=x", "--label", "k3=v3", imageName)

	actual := make(map[string]string)
	inspectFieldAndUnmarshall(c, name, "Config.Labels", &actual)

	if !reflect.DeepEqual(expected, actual) {
		c.Fatalf("Expected %s got %s", expected, actual)
	}
}

func (s *DockerCLICreateSuite) TestCreateHostnameWithNumber(c *testing.T) {
	imgName := "busybox"
	// Busybox on Windows does not implement hostname command
	if testEnv.DaemonInfo.OSType == "windows" {
		imgName = testEnv.PlatformDefaults.BaseImage
	}
	out := cli.DockerCmd(c, "run", "-h", "web.0", imgName, "hostname").Combined()
	assert.Equal(c, strings.TrimSpace(out), "web.0", "hostname not set, expected `web.0`, got: %s", out)
}

func (s *DockerCLICreateSuite) TestCreateRM(c *testing.T) {
	// Test to make sure we can 'rm' a new container that is in
	// "Created" state, and has ever been run. Test "rm -f" too.

	// create a container
	cID := cli.DockerCmd(c, "create", "busybox").Stdout()
	cID = strings.TrimSpace(cID)
	cli.DockerCmd(c, "rm", cID)

	// Now do it again so we can "rm -f" this time
	cID = cli.DockerCmd(c, "create", "busybox").Stdout()
	cID = strings.TrimSpace(cID)
	cli.DockerCmd(c, "rm", "-f", cID)
}

func (s *DockerCLICreateSuite) TestCreateModeIpcContainer(c *testing.T) {
	// Uses Linux specific functionality (--ipc)
	testRequires(c, DaemonIsLinux, testEnv.IsLocalDaemon)

	id := cli.DockerCmd(c, "create", "busybox").Stdout()
	id = strings.TrimSpace(id)

	cli.DockerCmd(c, "create", fmt.Sprintf("--ipc=container:%s", id), "busybox")
}

func (s *DockerCLICreateSuite) TestCreateStopSignal(c *testing.T) {
	const name = "test_create_stop_signal"
	cli.DockerCmd(c, "create", "--name", name, "--stop-signal", "9", "busybox")

	res := inspectFieldJSON(c, name, "Config.StopSignal")
	assert.Assert(c, is.Contains(res, "9"))
}

func (s *DockerCLICreateSuite) TestCreateWithWorkdir(c *testing.T) {
	const name = "foo"

	prefix, slash := getPrefixAndSlashFromDaemonPlatform()
	dir := prefix + slash + "home" + slash + "foo" + slash + "bar"

	cli.DockerCmd(c, "create", "--name", name, "-w", dir, "busybox")
	// Windows does not create the workdir until the container is started
	if testEnv.DaemonInfo.OSType == "windows" {
		cli.DockerCmd(c, "start", name)
		if testEnv.DaemonInfo.Isolation.IsHyperV() {
			// Hyper-V isolated containers do not allow file-operations on a
			// running container. This test currently uses `docker cp` to verify
			// that the WORKDIR was automatically created, which cannot be done
			// while the container is running.
			cli.DockerCmd(c, "stop", name)
		}
	}
	// TODO: rewrite this test to not use `docker cp` for verifying that the WORKDIR was created
	cli.DockerCmd(c, "cp", fmt.Sprintf("%s:%s", name, dir), prefix+slash+"tmp")
}

func (s *DockerCLICreateSuite) TestCreateWithInvalidLogOpts(c *testing.T) {
	const name = "test-invalidate-log-opts"
	out, _, err := dockerCmdWithError("create", "--name", name, "--log-opt", "invalid=true", "busybox")
	assert.ErrorContains(c, err, "")
	assert.Assert(c, is.Contains(out, "unknown log opt"))
	assert.Assert(c, is.Contains(out, "unknown log opt"))

	out = cli.DockerCmd(c, "ps", "-a").Stdout()
	assert.Assert(c, !strings.Contains(out, name))
}

// Test case for #23498
func (s *DockerCLICreateSuite) TestCreateUnsetEntrypoint(c *testing.T) {
	const name = "test-entrypoint"
	dockerfile := `FROM busybox
ADD entrypoint.sh /entrypoint.sh
RUN chmod 755 /entrypoint.sh
ENTRYPOINT ["/entrypoint.sh"]
CMD echo foobar`

	buildCtx := fakecontext.New(c, "",
		fakecontext.WithDockerfile(dockerfile),
		fakecontext.WithFiles(map[string]string{
			"entrypoint.sh": `#!/bin/sh
echo "I am an entrypoint"
exec "$@"`,
		}))

	cli.BuildCmd(c, name, build.WithExternalBuildContext(buildCtx))

	out := cli.DockerCmd(c, "create", "--entrypoint=", name, "echo", "foo").Combined()
	id := strings.TrimSpace(out)
	assert.Assert(c, id != "")
	out = cli.DockerCmd(c, "start", "-a", id).Combined()
	assert.Equal(c, strings.TrimSpace(out), "foo")
}

// #22471
func (s *DockerCLICreateSuite) TestCreateStopTimeout(c *testing.T) {
	name1 := "test_create_stop_timeout_1"
	cli.DockerCmd(c, "create", "--name", name1, "--stop-timeout", "15", "busybox")

	res := inspectFieldJSON(c, name1, "Config.StopTimeout")
	assert.Assert(c, is.Contains(res, "15"))
	name2 := "test_create_stop_timeout_2"
	cli.DockerCmd(c, "create", "--name", name2, "busybox")

	res = inspectFieldJSON(c, name2, "Config.StopTimeout")
	assert.Assert(c, is.Contains(res, "null"))
}
