package main

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/pkg/integration/checker"
	"github.com/docker/docker/runconfig"
	"github.com/go-check/check"
)

func checkValidGraphDriver(c *check.C, name string) {
	if name != "devicemapper" && name != "overlay" && name != "vfs" && name != "zfs" && name != "btrfs" && name != "aufs" {
		c.Fatalf("%v is not a valid graph driver name", name)
	}
}

func (s *DockerSuite) TestInspectImage(c *check.C) {
	testRequires(c, DaemonIsLinux)
	imageTest := "emptyfs"
	// It is important that this ID remain stable. If a code change causes
	// it to be different, this is equivalent to a cache bust when pulling
	// a legacy-format manifest. If the check at the end of this function
	// fails, fix the difference in the image serialization instead of
	// updating this hash.
	imageTestID := "sha256:11f64303f0f7ffdc71f001788132bca5346831939a956e3e975c93267d89a16d"
	id, err := inspectField(imageTest, "Id")
	c.Assert(err, checker.IsNil)

	c.Assert(id, checker.Equals, imageTestID)
}

func (s *DockerSuite) TestInspectInt64(c *check.C) {
	testRequires(c, DaemonIsLinux)

	dockerCmd(c, "run", "-d", "-m=300M", "--name", "inspectTest", "busybox", "true")
	inspectOut, err := inspectField("inspectTest", "HostConfig.Memory")
	c.Assert(err, check.IsNil)
	c.Assert(inspectOut, checker.Equals, "314572800")
}

func (s *DockerSuite) TestInspectDefault(c *check.C) {
	testRequires(c, DaemonIsLinux)
	//Both the container and image are named busybox. docker inspect will fetch the container JSON.
	//If the container JSON is not available, it will go for the image JSON.

	dockerCmd(c, "run", "--name=busybox", "-d", "busybox", "true")
	dockerCmd(c, "inspect", "busybox")
}

func (s *DockerSuite) TestInspectStatus(c *check.C) {
	defer unpauseAllContainers()
	testRequires(c, DaemonIsLinux)
	out, _ := dockerCmd(c, "run", "-d", "busybox", "top")
	out = strings.TrimSpace(out)

	inspectOut, err := inspectField(out, "State.Status")
	c.Assert(err, checker.IsNil)
	c.Assert(inspectOut, checker.Equals, "running")

	dockerCmd(c, "pause", out)
	inspectOut, err = inspectField(out, "State.Status")
	c.Assert(err, checker.IsNil)
	c.Assert(inspectOut, checker.Equals, "paused")

	dockerCmd(c, "unpause", out)
	inspectOut, err = inspectField(out, "State.Status")
	c.Assert(err, checker.IsNil)
	c.Assert(inspectOut, checker.Equals, "running")

	dockerCmd(c, "stop", out)
	inspectOut, err = inspectField(out, "State.Status")
	c.Assert(err, checker.IsNil)
	c.Assert(inspectOut, checker.Equals, "exited")

}

func (s *DockerSuite) TestInspectTypeFlagContainer(c *check.C) {
	testRequires(c, DaemonIsLinux)
	//Both the container and image are named busybox. docker inspect will fetch container
	//JSON State.Running field. If the field is true, it's a container.

	dockerCmd(c, "run", "--name=busybox", "-d", "busybox", "top")

	formatStr := "--format='{{.State.Running}}'"
	out, _ := dockerCmd(c, "inspect", "--type=container", formatStr, "busybox")
	c.Assert(out, checker.Equals, "true\n") // not a container JSON
}

func (s *DockerSuite) TestInspectTypeFlagWithNoContainer(c *check.C) {
	testRequires(c, DaemonIsLinux)
	//Run this test on an image named busybox. docker inspect will try to fetch container
	//JSON. Since there is no container named busybox and --type=container, docker inspect will
	//not try to get the image JSON. It will throw an error.

	dockerCmd(c, "run", "-d", "busybox", "true")

	_, _, err := dockerCmdWithError("inspect", "--type=container", "busybox")
	// docker inspect should fail, as there is no container named busybox
	c.Assert(err, checker.NotNil)
}

func (s *DockerSuite) TestInspectTypeFlagWithImage(c *check.C) {
	testRequires(c, DaemonIsLinux)
	//Both the container and image are named busybox. docker inspect will fetch image
	//JSON as --type=image. if there is no image with name busybox, docker inspect
	//will throw an error.

	dockerCmd(c, "run", "--name=busybox", "-d", "busybox", "true")

	out, _ := dockerCmd(c, "inspect", "--type=image", "busybox")
	c.Assert(out, checker.Not(checker.Contains), "State") // not an image JSON
}

func (s *DockerSuite) TestInspectTypeFlagWithInvalidValue(c *check.C) {
	testRequires(c, DaemonIsLinux)
	//Both the container and image are named busybox. docker inspect will fail
	//as --type=foobar is not a valid value for the flag.

	dockerCmd(c, "run", "--name=busybox", "-d", "busybox", "true")

	out, exitCode, err := dockerCmdWithError("inspect", "--type=foobar", "busybox")
	c.Assert(err, checker.NotNil, check.Commentf("%s", exitCode))
	c.Assert(exitCode, checker.Equals, 1, check.Commentf("%s", err))
	c.Assert(out, checker.Contains, "not a valid value for --type")
}

func (s *DockerSuite) TestInspectImageFilterInt(c *check.C) {
	testRequires(c, DaemonIsLinux)
	imageTest := "emptyfs"
	out, err := inspectField(imageTest, "Size")
	c.Assert(err, checker.IsNil)

	size, err := strconv.Atoi(out)
	c.Assert(err, checker.IsNil, check.Commentf("failed to inspect size of the image: %s, %v", out, err))

	//now see if the size turns out to be the same
	formatStr := fmt.Sprintf("--format='{{eq .Size %d}}'", size)
	out, _ = dockerCmd(c, "inspect", formatStr, imageTest)
	result, err := strconv.ParseBool(strings.TrimSuffix(out, "\n"))
	c.Assert(err, checker.IsNil)
	c.Assert(result, checker.Equals, true)
}

func (s *DockerSuite) TestInspectContainerFilterInt(c *check.C) {
	testRequires(c, DaemonIsLinux)
	runCmd := exec.Command(dockerBinary, "run", "-i", "-a", "stdin", "busybox", "cat")
	runCmd.Stdin = strings.NewReader("blahblah")
	out, _, _, err := runCommandWithStdoutStderr(runCmd)
	c.Assert(err, checker.IsNil, check.Commentf("failed to run container: %v, output: %q", err, out))

	id := strings.TrimSpace(out)

	out, err = inspectField(id, "State.ExitCode")
	c.Assert(err, checker.IsNil)

	exitCode, err := strconv.Atoi(out)
	c.Assert(err, checker.IsNil, check.Commentf("failed to inspect exitcode of the container: %s, %v", out, err))

	//now get the exit code to verify
	formatStr := fmt.Sprintf("--format='{{eq .State.ExitCode %d}}'", exitCode)
	out, _ = dockerCmd(c, "inspect", formatStr, id)
	result, err := strconv.ParseBool(strings.TrimSuffix(out, "\n"))
	c.Assert(err, checker.IsNil)
	c.Assert(result, checker.Equals, true)
}

func (s *DockerSuite) TestInspectImageGraphDriver(c *check.C) {
	testRequires(c, DaemonIsLinux)
	imageTest := "emptyfs"
	name, err := inspectField(imageTest, "GraphDriver.Name")
	c.Assert(err, checker.IsNil)

	checkValidGraphDriver(c, name)

	if name != "devicemapper" {
		c.Skip("requires devicemapper graphdriver")
	}

	deviceID, err := inspectField(imageTest, "GraphDriver.Data.DeviceId")
	c.Assert(err, checker.IsNil)

	_, err = strconv.Atoi(deviceID)
	c.Assert(err, checker.IsNil, check.Commentf("failed to inspect DeviceId of the image: %s, %v", deviceID, err))

	deviceSize, err := inspectField(imageTest, "GraphDriver.Data.DeviceSize")
	c.Assert(err, checker.IsNil)

	_, err = strconv.ParseUint(deviceSize, 10, 64)
	c.Assert(err, checker.IsNil, check.Commentf("failed to inspect DeviceSize of the image: %s, %v", deviceSize, err))
}

func (s *DockerSuite) TestInspectContainerGraphDriver(c *check.C) {
	testRequires(c, DaemonIsLinux)
	out, _ := dockerCmd(c, "run", "-d", "busybox", "true")
	out = strings.TrimSpace(out)

	name, err := inspectField(out, "GraphDriver.Name")
	c.Assert(err, checker.IsNil)

	checkValidGraphDriver(c, name)

	if name != "devicemapper" {
		return
	}

	deviceID, err := inspectField(out, "GraphDriver.Data.DeviceId")
	c.Assert(err, checker.IsNil)

	_, err = strconv.Atoi(deviceID)
	c.Assert(err, checker.IsNil, check.Commentf("failed to inspect DeviceId of the image: %s, %v", deviceID, err))

	deviceSize, err := inspectField(out, "GraphDriver.Data.DeviceSize")
	c.Assert(err, checker.IsNil)

	_, err = strconv.ParseUint(deviceSize, 10, 64)
	c.Assert(err, checker.IsNil, check.Commentf("failed to inspect DeviceSize of the image: %s, %v", deviceSize, err))
}

func (s *DockerSuite) TestInspectBindMountPoint(c *check.C) {
	testRequires(c, DaemonIsLinux)
	dockerCmd(c, "run", "-d", "--name", "test", "-v", "/data:/data:ro,z", "busybox", "cat")

	vol, err := inspectFieldJSON("test", "Mounts")
	c.Assert(err, checker.IsNil)

	var mp []types.MountPoint
	err = unmarshalJSON([]byte(vol), &mp)
	c.Assert(err, checker.IsNil)

	// check that there is only one mountpoint
	c.Assert(mp, check.HasLen, 1)

	m := mp[0]

	c.Assert(m.Name, checker.Equals, "")
	c.Assert(m.Driver, checker.Equals, "")
	c.Assert(m.Source, checker.Equals, "/data")
	c.Assert(m.Destination, checker.Equals, "/data")
	c.Assert(m.Mode, checker.Equals, "ro,z")
	c.Assert(m.RW, checker.Equals, false)
}

// #14947
func (s *DockerSuite) TestInspectTimesAsRFC3339Nano(c *check.C) {
	testRequires(c, DaemonIsLinux)
	out, _ := dockerCmd(c, "run", "-d", "busybox", "true")
	id := strings.TrimSpace(out)
	startedAt, err := inspectField(id, "State.StartedAt")
	c.Assert(err, checker.IsNil)
	finishedAt, err := inspectField(id, "State.FinishedAt")
	c.Assert(err, checker.IsNil)
	created, err := inspectField(id, "Created")
	c.Assert(err, checker.IsNil)

	_, err = time.Parse(time.RFC3339Nano, startedAt)
	c.Assert(err, checker.IsNil)
	_, err = time.Parse(time.RFC3339Nano, finishedAt)
	c.Assert(err, checker.IsNil)
	_, err = time.Parse(time.RFC3339Nano, created)
	c.Assert(err, checker.IsNil)

	created, err = inspectField("busybox", "Created")
	c.Assert(err, checker.IsNil)

	_, err = time.Parse(time.RFC3339Nano, created)
	c.Assert(err, checker.IsNil)
}

// #15633
func (s *DockerSuite) TestInspectLogConfigNoType(c *check.C) {
	testRequires(c, DaemonIsLinux)
	dockerCmd(c, "create", "--name=test", "--log-opt", "max-file=42", "busybox")
	var logConfig runconfig.LogConfig

	out, err := inspectFieldJSON("test", "HostConfig.LogConfig")
	c.Assert(err, checker.IsNil, check.Commentf("%v", out))

	err = json.NewDecoder(strings.NewReader(out)).Decode(&logConfig)
	c.Assert(err, checker.IsNil, check.Commentf("%v", out))

	c.Assert(logConfig.Type, checker.Equals, "json-file")
	c.Assert(logConfig.Config["max-file"], checker.Equals, "42", check.Commentf("%v", logConfig))
}

func (s *DockerSuite) TestInspectNoSizeFlagContainer(c *check.C) {

	//Both the container and image are named busybox. docker inspect will fetch container
	//JSON SizeRw and SizeRootFs field. If there is no flag --size/-s, there are no size fields.

	dockerCmd(c, "run", "--name=busybox", "-d", "busybox", "top")

	formatStr := "--format='{{.SizeRw}},{{.SizeRootFs}}'"
	out, _ := dockerCmd(c, "inspect", "--type=container", formatStr, "busybox")
	c.Assert(strings.TrimSpace(out), check.Equals, "<nil>,<nil>", check.Commentf("Exepcted not to display size info: %s", out))
}

func (s *DockerSuite) TestInspectSizeFlagContainer(c *check.C) {
	dockerCmd(c, "run", "--name=busybox", "-d", "busybox", "top")

	formatStr := "--format='{{.SizeRw}},{{.SizeRootFs}}'"
	out, _ := dockerCmd(c, "inspect", "-s", "--type=container", formatStr, "busybox")
	sz := strings.Split(out, ",")

	c.Assert(strings.TrimSpace(sz[0]), check.Not(check.Equals), "<nil>")
	c.Assert(strings.TrimSpace(sz[1]), check.Not(check.Equals), "<nil>")
}

func (s *DockerSuite) TestInspectSizeFlagImage(c *check.C) {
	dockerCmd(c, "run", "--name=busybox", "-d", "busybox", "top")

	formatStr := "--format='{{.SizeRw}},{{.SizeRootFs}}'"
	out, _, err := dockerCmdWithError("inspect", "-s", "--type=image", formatStr, "busybox")

	// Template error rather than <no value>
	// This is a more correct behavior because images don't have sizes associated.
	c.Assert(err, check.Not(check.IsNil))
	c.Assert(out, checker.Contains, "Template parsing error")
}

func (s *DockerSuite) TestInspectTempateError(c *check.C) {
	//Both the container and image are named busybox. docker inspect will fetch container
	//JSON State.Running field. If the field is true, it's a container.

	dockerCmd(c, "run", "--name=busybox", "-d", "busybox", "top")

	out, _, err := dockerCmdWithError("inspect", "--type=container", "--format='Format container: {{.ThisDoesNotExist}}'", "busybox")

	c.Assert(err, check.Not(check.IsNil))
	c.Assert(out, checker.Contains, "Template parsing error")
}

func (s *DockerSuite) TestInspectJSONFields(c *check.C) {
	dockerCmd(c, "run", "--name=busybox", "-d", "busybox", "top")
	out, _, err := dockerCmdWithError("inspect", "--type=container", "--format='{{.HostConfig.Dns}}'", "busybox")

	c.Assert(err, check.IsNil)
	c.Assert(out, checker.Equals, "[]\n")
}
