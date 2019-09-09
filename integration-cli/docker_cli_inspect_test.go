package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/integration-cli/checker"
	"github.com/go-check/check"
	"gotest.tools/assert"
	"gotest.tools/icmd"
)

func checkValidGraphDriver(c *testing.T, name string) {
	if name != "devicemapper" && name != "overlay" && name != "vfs" && name != "zfs" && name != "btrfs" && name != "aufs" {
		c.Fatalf("%v is not a valid graph driver name", name)
	}
}

func (s *DockerSuite) TestInspectImage(c *testing.T) {
	testRequires(c, DaemonIsLinux)
	imageTest := "emptyfs"
	// It is important that this ID remain stable. If a code change causes
	// it to be different, this is equivalent to a cache bust when pulling
	// a legacy-format manifest. If the check at the end of this function
	// fails, fix the difference in the image serialization instead of
	// updating this hash.
	imageTestID := "sha256:11f64303f0f7ffdc71f001788132bca5346831939a956e3e975c93267d89a16d"
	id := inspectField(c, imageTest, "Id")

	assert.Assert(c, id, checker.Equals, imageTestID)
}

func (s *DockerSuite) TestInspectInt64(c *testing.T) {
	dockerCmd(c, "run", "-d", "-m=300M", "--name", "inspectTest", "busybox", "true")
	inspectOut := inspectField(c, "inspectTest", "HostConfig.Memory")
	assert.Assert(c, inspectOut, checker.Equals, "314572800")
}

func (s *DockerSuite) TestInspectDefault(c *testing.T) {
	//Both the container and image are named busybox. docker inspect will fetch the container JSON.
	//If the container JSON is not available, it will go for the image JSON.

	out, _ := dockerCmd(c, "run", "--name=busybox", "-d", "busybox", "true")
	containerID := strings.TrimSpace(out)

	inspectOut := inspectField(c, "busybox", "Id")
	assert.Assert(c, strings.TrimSpace(inspectOut), checker.Equals, containerID)
}

func (s *DockerSuite) TestInspectStatus(c *testing.T) {
	out := runSleepingContainer(c, "-d")
	out = strings.TrimSpace(out)

	inspectOut := inspectField(c, out, "State.Status")
	assert.Assert(c, inspectOut, checker.Equals, "running")

	// Windows does not support pause/unpause on Windows Server Containers.
	// (RS1 does for Hyper-V Containers, but production CI is not setup for that)
	if testEnv.OSType != "windows" {
		dockerCmd(c, "pause", out)
		inspectOut = inspectField(c, out, "State.Status")
		assert.Assert(c, inspectOut, checker.Equals, "paused")

		dockerCmd(c, "unpause", out)
		inspectOut = inspectField(c, out, "State.Status")
		assert.Assert(c, inspectOut, checker.Equals, "running")
	}

	dockerCmd(c, "stop", out)
	inspectOut = inspectField(c, out, "State.Status")
	assert.Assert(c, inspectOut, checker.Equals, "exited")

}

func (s *DockerSuite) TestInspectTypeFlagContainer(c *testing.T) {
	//Both the container and image are named busybox. docker inspect will fetch container
	//JSON State.Running field. If the field is true, it's a container.
	runSleepingContainer(c, "--name=busybox", "-d")

	formatStr := "--format={{.State.Running}}"
	out, _ := dockerCmd(c, "inspect", "--type=container", formatStr, "busybox")
	assert.Equal(c, out, "true\n") // not a container JSON
}

func (s *DockerSuite) TestInspectTypeFlagWithNoContainer(c *testing.T) {
	//Run this test on an image named busybox. docker inspect will try to fetch container
	//JSON. Since there is no container named busybox and --type=container, docker inspect will
	//not try to get the image JSON. It will throw an error.

	dockerCmd(c, "run", "-d", "busybox", "true")

	_, _, err := dockerCmdWithError("inspect", "--type=container", "busybox")
	// docker inspect should fail, as there is no container named busybox
	assert.ErrorContains(c, err, "")
}

func (s *DockerSuite) TestInspectTypeFlagWithImage(c *testing.T) {
	//Both the container and image are named busybox. docker inspect will fetch image
	//JSON as --type=image. if there is no image with name busybox, docker inspect
	//will throw an error.

	dockerCmd(c, "run", "--name=busybox", "-d", "busybox", "true")

	out, _ := dockerCmd(c, "inspect", "--type=image", "busybox")
	// not an image JSON
	assert.Assert(c, out, checker.Not(checker.Contains), "State")
}

func (s *DockerSuite) TestInspectTypeFlagWithInvalidValue(c *testing.T) {
	//Both the container and image are named busybox. docker inspect will fail
	//as --type=foobar is not a valid value for the flag.

	dockerCmd(c, "run", "--name=busybox", "-d", "busybox", "true")

	out, exitCode, err := dockerCmdWithError("inspect", "--type=foobar", "busybox")
	assert.Assert(c, err, checker.NotNil, check.Commentf("%d", exitCode))
	assert.Assert(c, exitCode, checker.Equals, 1, check.Commentf("%s", err))
	assert.Assert(c, out, checker.Contains, "not a valid value for --type")
}

func (s *DockerSuite) TestInspectImageFilterInt(c *testing.T) {
	testRequires(c, DaemonIsLinux)
	imageTest := "emptyfs"
	out := inspectField(c, imageTest, "Size")

	size, err := strconv.Atoi(out)
	assert.Assert(c, err, checker.IsNil, check.Commentf("failed to inspect size of the image: %s, %v", out, err))

	//now see if the size turns out to be the same
	formatStr := fmt.Sprintf("--format={{eq .Size %d}}", size)
	out, _ = dockerCmd(c, "inspect", formatStr, imageTest)
	result, err := strconv.ParseBool(strings.TrimSuffix(out, "\n"))
	assert.NilError(c, err)
	assert.Assert(c, result, checker.Equals, true)
}

func (s *DockerSuite) TestInspectContainerFilterInt(c *testing.T) {
	result := icmd.RunCmd(icmd.Cmd{
		Command: []string{dockerBinary, "run", "-i", "-a", "stdin", "busybox", "cat"},
		Stdin:   strings.NewReader("blahblah"),
	})
	result.Assert(c, icmd.Success)
	out := result.Stdout()
	id := strings.TrimSpace(out)

	out = inspectField(c, id, "State.ExitCode")

	exitCode, err := strconv.Atoi(out)
	assert.Assert(c, err, checker.IsNil, check.Commentf("failed to inspect exitcode of the container: %s, %v", out, err))

	//now get the exit code to verify
	formatStr := fmt.Sprintf("--format={{eq .State.ExitCode %d}}", exitCode)
	out, _ = dockerCmd(c, "inspect", formatStr, id)
	inspectResult, err := strconv.ParseBool(strings.TrimSuffix(out, "\n"))
	assert.NilError(c, err)
	assert.Assert(c, inspectResult, checker.Equals, true)
}

func (s *DockerSuite) TestInspectImageGraphDriver(c *testing.T) {
	testRequires(c, DaemonIsLinux, Devicemapper)
	imageTest := "emptyfs"
	name := inspectField(c, imageTest, "GraphDriver.Name")

	checkValidGraphDriver(c, name)

	deviceID := inspectField(c, imageTest, "GraphDriver.Data.DeviceId")

	_, err := strconv.Atoi(deviceID)
	assert.Assert(c, err, checker.IsNil, check.Commentf("failed to inspect DeviceId of the image: %s, %v", deviceID, err))

	deviceSize := inspectField(c, imageTest, "GraphDriver.Data.DeviceSize")

	_, err = strconv.ParseUint(deviceSize, 10, 64)
	assert.Assert(c, err, checker.IsNil, check.Commentf("failed to inspect DeviceSize of the image: %s, %v", deviceSize, err))
}

func (s *DockerSuite) TestInspectContainerGraphDriver(c *testing.T) {
	testRequires(c, DaemonIsLinux, Devicemapper)

	out, _ := dockerCmd(c, "run", "-d", "busybox", "true")
	out = strings.TrimSpace(out)

	name := inspectField(c, out, "GraphDriver.Name")

	checkValidGraphDriver(c, name)

	imageDeviceID := inspectField(c, "busybox", "GraphDriver.Data.DeviceId")

	deviceID := inspectField(c, out, "GraphDriver.Data.DeviceId")

	assert.Assert(c, imageDeviceID != deviceID)

	_, err := strconv.Atoi(deviceID)
	assert.Assert(c, err, checker.IsNil, check.Commentf("failed to inspect DeviceId of the image: %s, %v", deviceID, err))

	deviceSize := inspectField(c, out, "GraphDriver.Data.DeviceSize")

	_, err = strconv.ParseUint(deviceSize, 10, 64)
	assert.Assert(c, err, checker.IsNil, check.Commentf("failed to inspect DeviceSize of the image: %s, %v", deviceSize, err))
}

func (s *DockerSuite) TestInspectBindMountPoint(c *testing.T) {
	modifier := ",z"
	prefix, slash := getPrefixAndSlashFromDaemonPlatform()
	if testEnv.OSType == "windows" {
		modifier = ""
		// Linux creates the host directory if it doesn't exist. Windows does not.
		os.Mkdir(`c:\data`, os.ModeDir)
	}

	dockerCmd(c, "run", "-d", "--name", "test", "-v", prefix+slash+"data:"+prefix+slash+"data:ro"+modifier, "busybox", "cat")

	vol := inspectFieldJSON(c, "test", "Mounts")

	var mp []types.MountPoint
	err := json.Unmarshal([]byte(vol), &mp)
	assert.NilError(c, err)

	// check that there is only one mountpoint
	assert.Assert(c, mp, checker.HasLen, 1)

	m := mp[0]

	assert.Assert(c, m.Name, checker.Equals, "")
	assert.Assert(c, m.Driver, checker.Equals, "")
	assert.Assert(c, m.Source, checker.Equals, prefix+slash+"data")
	assert.Assert(c, m.Destination, checker.Equals, prefix+slash+"data")
	if testEnv.OSType != "windows" { // Windows does not set mode
		assert.Assert(c, m.Mode, checker.Equals, "ro"+modifier)
	}
	assert.Assert(c, m.RW, checker.Equals, false)
}

func (s *DockerSuite) TestInspectNamedMountPoint(c *testing.T) {
	prefix, slash := getPrefixAndSlashFromDaemonPlatform()

	dockerCmd(c, "run", "-d", "--name", "test", "-v", "data:"+prefix+slash+"data", "busybox", "cat")

	vol := inspectFieldJSON(c, "test", "Mounts")

	var mp []types.MountPoint
	err := json.Unmarshal([]byte(vol), &mp)
	assert.NilError(c, err)

	// check that there is only one mountpoint
	assert.Assert(c, mp, checker.HasLen, 1)

	m := mp[0]

	assert.Assert(c, m.Name, checker.Equals, "data")
	assert.Assert(c, m.Driver, checker.Equals, "local")
	assert.Assert(c, m.Source != "")
	assert.Assert(c, m.Destination, checker.Equals, prefix+slash+"data")
	assert.Assert(c, m.RW, checker.Equals, true)
}

// #14947
func (s *DockerSuite) TestInspectTimesAsRFC3339Nano(c *testing.T) {
	out, _ := dockerCmd(c, "run", "-d", "busybox", "true")
	id := strings.TrimSpace(out)
	startedAt := inspectField(c, id, "State.StartedAt")
	finishedAt := inspectField(c, id, "State.FinishedAt")
	created := inspectField(c, id, "Created")

	_, err := time.Parse(time.RFC3339Nano, startedAt)
	assert.NilError(c, err)
	_, err = time.Parse(time.RFC3339Nano, finishedAt)
	assert.NilError(c, err)
	_, err = time.Parse(time.RFC3339Nano, created)
	assert.NilError(c, err)

	created = inspectField(c, "busybox", "Created")

	_, err = time.Parse(time.RFC3339Nano, created)
	assert.NilError(c, err)
}

// #15633
func (s *DockerSuite) TestInspectLogConfigNoType(c *testing.T) {
	dockerCmd(c, "create", "--name=test", "--log-opt", "max-file=42", "busybox")
	var logConfig container.LogConfig

	out := inspectFieldJSON(c, "test", "HostConfig.LogConfig")

	err := json.NewDecoder(strings.NewReader(out)).Decode(&logConfig)
	assert.Assert(c, err, checker.IsNil, check.Commentf("%v", out))

	assert.Assert(c, logConfig.Type, checker.Equals, "json-file")
	assert.Assert(c, logConfig.Config["max-file"], checker.Equals, "42", check.Commentf("%v", logConfig))
}

func (s *DockerSuite) TestInspectNoSizeFlagContainer(c *testing.T) {

	//Both the container and image are named busybox. docker inspect will fetch container
	//JSON SizeRw and SizeRootFs field. If there is no flag --size/-s, there are no size fields.

	runSleepingContainer(c, "--name=busybox", "-d")

	formatStr := "--format={{.SizeRw}},{{.SizeRootFs}}"
	out, _ := dockerCmd(c, "inspect", "--type=container", formatStr, "busybox")
	assert.Assert(c, strings.TrimSpace(out), checker.Equals, "<nil>,<nil>", check.Commentf("Expected not to display size info: %s", out))
}

func (s *DockerSuite) TestInspectSizeFlagContainer(c *testing.T) {
	runSleepingContainer(c, "--name=busybox", "-d")

	formatStr := "--format='{{.SizeRw}},{{.SizeRootFs}}'"
	out, _ := dockerCmd(c, "inspect", "-s", "--type=container", formatStr, "busybox")
	sz := strings.Split(out, ",")

	assert.Assert(c, strings.TrimSpace(sz[0]) != "<nil>")
	assert.Assert(c, strings.TrimSpace(sz[1]) != "<nil>")
}

func (s *DockerSuite) TestInspectTemplateError(c *testing.T) {
	// Template parsing error for both the container and image.

	runSleepingContainer(c, "--name=container1", "-d")

	out, _, err := dockerCmdWithError("inspect", "--type=container", "--format='Format container: {{.ThisDoesNotExist}}'", "container1")
	assert.Assert(c, err != nil)
	assert.Assert(c, out, checker.Contains, "Template parsing error")

	out, _, err = dockerCmdWithError("inspect", "--type=image", "--format='Format container: {{.ThisDoesNotExist}}'", "busybox")
	assert.Assert(c, err != nil)
	assert.Assert(c, out, checker.Contains, "Template parsing error")
}

func (s *DockerSuite) TestInspectJSONFields(c *testing.T) {
	runSleepingContainer(c, "--name=busybox", "-d")
	out, _, err := dockerCmdWithError("inspect", "--type=container", "--format={{.HostConfig.Dns}}", "busybox")

	assert.NilError(c, err)
	assert.Equal(c, out, "[]\n")
}

func (s *DockerSuite) TestInspectByPrefix(c *testing.T) {
	id := inspectField(c, "busybox", "Id")
	assert.Assert(c, strings.HasPrefix(id, "sha256:"))

	id2 := inspectField(c, id[:12], "Id")
	assert.Assert(c, id, checker.Equals, id2)

	id3 := inspectField(c, strings.TrimPrefix(id, "sha256:")[:12], "Id")
	assert.Assert(c, id, checker.Equals, id3)
}

func (s *DockerSuite) TestInspectStopWhenNotFound(c *testing.T) {
	runSleepingContainer(c, "--name=busybox1", "-d")
	runSleepingContainer(c, "--name=busybox2", "-d")
	result := dockerCmdWithResult("inspect", "--type=container", "--format='{{.Name}}'", "busybox1", "busybox2", "missing")

	assert.Assert(c, result.Error != nil)
	assert.Assert(c, result.Stdout(), checker.Contains, "busybox1")
	assert.Assert(c, result.Stdout(), checker.Contains, "busybox2")
	assert.Assert(c, result.Stderr(), checker.Contains, "Error: No such container: missing")

	// test inspect would not fast fail
	result = dockerCmdWithResult("inspect", "--type=container", "--format='{{.Name}}'", "missing", "busybox1", "busybox2")

	assert.Assert(c, result.Error != nil)
	assert.Assert(c, result.Stdout(), checker.Contains, "busybox1")
	assert.Assert(c, result.Stdout(), checker.Contains, "busybox2")
	assert.Assert(c, result.Stderr(), checker.Contains, "Error: No such container: missing")
}

func (s *DockerSuite) TestInspectHistory(c *testing.T) {
	dockerCmd(c, "run", "--name=testcont", "busybox", "echo", "hello")
	dockerCmd(c, "commit", "-m", "test comment", "testcont", "testimg")
	out, _, err := dockerCmdWithError("inspect", "--format='{{.Comment}}'", "testimg")
	assert.NilError(c, err)
	assert.Assert(c, out, checker.Contains, "test comment")
}

func (s *DockerSuite) TestInspectContainerNetworkDefault(c *testing.T) {
	testRequires(c, DaemonIsLinux)

	contName := "test1"
	dockerCmd(c, "run", "--name", contName, "-d", "busybox", "top")
	netOut, _ := dockerCmd(c, "network", "inspect", "--format={{.ID}}", "bridge")
	out := inspectField(c, contName, "NetworkSettings.Networks")
	assert.Assert(c, out, checker.Contains, "bridge")
	out = inspectField(c, contName, "NetworkSettings.Networks.bridge.NetworkID")
	assert.Equal(c, strings.TrimSpace(out), strings.TrimSpace(netOut))
}

func (s *DockerSuite) TestInspectContainerNetworkCustom(c *testing.T) {
	testRequires(c, DaemonIsLinux)

	netOut, _ := dockerCmd(c, "network", "create", "net1")
	dockerCmd(c, "run", "--name=container1", "--net=net1", "-d", "busybox", "top")
	out := inspectField(c, "container1", "NetworkSettings.Networks")
	assert.Assert(c, out, checker.Contains, "net1")
	out = inspectField(c, "container1", "NetworkSettings.Networks.net1.NetworkID")
	assert.Equal(c, strings.TrimSpace(out), strings.TrimSpace(netOut))
}

func (s *DockerSuite) TestInspectRootFS(c *testing.T) {
	out, _, err := dockerCmdWithError("inspect", "busybox")
	assert.NilError(c, err)

	var imageJSON []types.ImageInspect
	err = json.Unmarshal([]byte(out), &imageJSON)
	assert.NilError(c, err)
	assert.Assert(c, len(imageJSON[0].RootFS.Layers) >= 1)
}

func (s *DockerSuite) TestInspectAmpersand(c *testing.T) {
	testRequires(c, DaemonIsLinux)

	name := "test"
	out, _ := dockerCmd(c, "run", "--name", name, "--env", `TEST_ENV="soanni&rtr"`, "busybox", "env")
	assert.Assert(c, out, checker.Contains, `soanni&rtr`)
	out, _ = dockerCmd(c, "inspect", name)
	assert.Assert(c, out, checker.Contains, `soanni&rtr`)
}

func (s *DockerSuite) TestInspectPlugin(c *testing.T) {
	testRequires(c, DaemonIsLinux, IsAmd64, Network)
	_, _, err := dockerCmdWithError("plugin", "install", "--grant-all-permissions", pNameWithTag)
	assert.NilError(c, err)

	out, _, err := dockerCmdWithError("inspect", "--type", "plugin", "--format", "{{.Name}}", pNameWithTag)
	assert.NilError(c, err)
	assert.Equal(c, strings.TrimSpace(out), pNameWithTag)

	out, _, err = dockerCmdWithError("inspect", "--format", "{{.Name}}", pNameWithTag)
	assert.NilError(c, err)
	assert.Equal(c, strings.TrimSpace(out), pNameWithTag)

	// Even without tag the inspect still work
	out, _, err = dockerCmdWithError("inspect", "--type", "plugin", "--format", "{{.Name}}", pNameWithTag)
	assert.NilError(c, err)
	assert.Equal(c, strings.TrimSpace(out), pNameWithTag)

	out, _, err = dockerCmdWithError("inspect", "--format", "{{.Name}}", pNameWithTag)
	assert.NilError(c, err)
	assert.Equal(c, strings.TrimSpace(out), pNameWithTag)

	_, _, err = dockerCmdWithError("plugin", "disable", pNameWithTag)
	assert.NilError(c, err)

	out, _, err = dockerCmdWithError("plugin", "remove", pNameWithTag)
	assert.NilError(c, err)
	assert.Assert(c, out, checker.Contains, pNameWithTag)
}

// Test case for 29185
func (s *DockerSuite) TestInspectUnknownObject(c *testing.T) {
	// This test should work on both Windows and Linux
	out, _, err := dockerCmdWithError("inspect", "foobar")
	assert.ErrorContains(c, err, "")
	assert.Assert(c, out, checker.Contains, "Error: No such object: foobar")
	assert.ErrorContains(c, err, "Error: No such object: foobar")
}
