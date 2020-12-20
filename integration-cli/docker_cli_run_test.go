package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Microsoft/hcsshim/osversion"
	"github.com/docker/docker/client"
	"github.com/docker/docker/integration-cli/cli"
	"github.com/docker/docker/integration-cli/cli/build"
	"github.com/docker/docker/pkg/stringid"
	"github.com/docker/docker/runconfig"
	"github.com/docker/docker/testutil"
	"github.com/docker/docker/testutil/fakecontext"
	"github.com/docker/go-connections/nat"
	"github.com/docker/libnetwork/resolvconf"
	"github.com/docker/libnetwork/types"
	"github.com/moby/sys/mountinfo"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/icmd"
)

// "test123" should be printed by docker run
func (s *DockerSuite) TestRunEchoStdout(c *testing.T) {
	out, _ := dockerCmd(c, "run", "busybox", "echo", "test123")
	if out != "test123\n" {
		c.Fatalf("container should've printed 'test123', got '%s'", out)
	}
}

// "test" should be printed
func (s *DockerSuite) TestRunEchoNamedContainer(c *testing.T) {
	out, _ := dockerCmd(c, "run", "--name", "testfoonamedcontainer", "busybox", "echo", "test")
	if out != "test\n" {
		c.Errorf("container should've printed 'test'")
	}
}

// docker run should not leak file descriptors. This test relies on Unix
// specific functionality and cannot run on Windows.
func (s *DockerSuite) TestRunLeakyFileDescriptors(c *testing.T) {
	testRequires(c, DaemonIsLinux)
	out, _ := dockerCmd(c, "run", "busybox", "ls", "-C", "/proc/self/fd")

	// normally, we should only get 0, 1, and 2, but 3 gets created by "ls" when it does "opendir" on the "fd" directory
	if out != "0  1  2  3\n" {
		c.Errorf("container should've printed '0  1  2  3', not: %s", out)
	}
}

// it should be possible to lookup Google DNS
// this will fail when Internet access is unavailable
func (s *DockerSuite) TestRunLookupGoogleDNS(c *testing.T) {
	testRequires(c, Network, NotArm)
	if testEnv.OSType == "windows" {
		// nslookup isn't present in Windows busybox. Is built-in. Further,
		// nslookup isn't present in nanoserver. Hence just use PowerShell...
		dockerCmd(c, "run", testEnv.PlatformDefaults.BaseImage, "powershell", "Resolve-DNSName", "google.com")
	} else {
		dockerCmd(c, "run", "busybox", "nslookup", "google.com")
	}

}

// the exit code should be 0
func (s *DockerSuite) TestRunExitCodeZero(c *testing.T) {
	dockerCmd(c, "run", "busybox", "true")
}

// the exit code should be 1
func (s *DockerSuite) TestRunExitCodeOne(c *testing.T) {
	_, exitCode, err := dockerCmdWithError("run", "busybox", "false")
	assert.ErrorContains(c, err, "")
	assert.Equal(c, exitCode, 1)
}

// it should be possible to pipe in data via stdin to a process running in a container
func (s *DockerSuite) TestRunStdinPipe(c *testing.T) {
	// TODO Windows: This needs some work to make compatible.
	testRequires(c, DaemonIsLinux)
	result := icmd.RunCmd(icmd.Cmd{
		Command: []string{dockerBinary, "run", "-i", "-a", "stdin", "busybox", "cat"},
		Stdin:   strings.NewReader("blahblah"),
	})
	result.Assert(c, icmd.Success)
	out := result.Stdout()

	out = strings.TrimSpace(out)
	dockerCmd(c, "wait", out)

	logsOut, _ := dockerCmd(c, "logs", out)

	containerLogs := strings.TrimSpace(logsOut)
	if containerLogs != "blahblah" {
		c.Errorf("logs didn't print the container's logs %s", containerLogs)
	}

	dockerCmd(c, "rm", out)
}

// the container's ID should be printed when starting a container in detached mode
func (s *DockerSuite) TestRunDetachedContainerIDPrinting(c *testing.T) {
	out, _ := dockerCmd(c, "run", "-d", "busybox", "true")

	out = strings.TrimSpace(out)
	dockerCmd(c, "wait", out)

	rmOut, _ := dockerCmd(c, "rm", out)

	rmOut = strings.TrimSpace(rmOut)
	if rmOut != out {
		c.Errorf("rm didn't print the container ID %s %s", out, rmOut)
	}
}

// the working directory should be set correctly
func (s *DockerSuite) TestRunWorkingDirectory(c *testing.T) {
	dir := "/root"
	image := "busybox"
	if testEnv.OSType == "windows" {
		dir = `C:/Windows`
	}

	// First with -w
	out, _ := dockerCmd(c, "run", "-w", dir, image, "pwd")
	out = strings.TrimSpace(out)
	if out != dir {
		c.Errorf("-w failed to set working directory")
	}

	// Then with --workdir
	out, _ = dockerCmd(c, "run", "--workdir", dir, image, "pwd")
	out = strings.TrimSpace(out)
	if out != dir {
		c.Errorf("--workdir failed to set working directory")
	}
}

// pinging Google's DNS resolver should fail when we disable the networking
func (s *DockerSuite) TestRunWithoutNetworking(c *testing.T) {
	count := "-c"
	image := "busybox"
	if testEnv.OSType == "windows" {
		count = "-n"
		image = testEnv.PlatformDefaults.BaseImage
	}

	// First using the long form --net
	out, exitCode, err := dockerCmdWithError("run", "--net=none", image, "ping", count, "1", "8.8.8.8")
	if err != nil && exitCode != 1 {
		c.Fatal(out, err)
	}
	if exitCode != 1 {
		c.Errorf("--net=none should've disabled the network; the container shouldn't have been able to ping 8.8.8.8")
	}
}

// test --link use container name to link target
func (s *DockerSuite) TestRunLinksContainerWithContainerName(c *testing.T) {
	// TODO Windows: This test cannot run on a Windows daemon as the networking
	// settings are not populated back yet on inspect.
	testRequires(c, DaemonIsLinux)
	dockerCmd(c, "run", "-i", "-t", "-d", "--name", "parent", "busybox")

	ip := inspectField(c, "parent", "NetworkSettings.Networks.bridge.IPAddress")

	out, _ := dockerCmd(c, "run", "--link", "parent:test", "busybox", "/bin/cat", "/etc/hosts")
	if !strings.Contains(out, ip+"	test") {
		c.Fatalf("use a container name to link target failed")
	}
}

// test --link use container id to link target
func (s *DockerSuite) TestRunLinksContainerWithContainerID(c *testing.T) {
	// TODO Windows: This test cannot run on a Windows daemon as the networking
	// settings are not populated back yet on inspect.
	testRequires(c, DaemonIsLinux)
	cID, _ := dockerCmd(c, "run", "-i", "-t", "-d", "busybox")

	cID = strings.TrimSpace(cID)
	ip := inspectField(c, cID, "NetworkSettings.Networks.bridge.IPAddress")

	out, _ := dockerCmd(c, "run", "--link", cID+":test", "busybox", "/bin/cat", "/etc/hosts")
	if !strings.Contains(out, ip+"	test") {
		c.Fatalf("use a container id to link target failed")
	}
}

func (s *DockerSuite) TestUserDefinedNetworkLinks(c *testing.T) {
	testRequires(c, DaemonIsLinux, NotUserNamespace, NotArm)
	dockerCmd(c, "network", "create", "-d", "bridge", "udlinkNet")

	dockerCmd(c, "run", "-d", "--net=udlinkNet", "--name=first", "busybox", "top")
	assert.Assert(c, waitRun("first") == nil)

	// run a container in user-defined network udlinkNet with a link for an existing container
	// and a link for a container that doesn't exist
	dockerCmd(c, "run", "-d", "--net=udlinkNet", "--name=second", "--link=first:foo",
		"--link=third:bar", "busybox", "top")
	assert.Assert(c, waitRun("second") == nil)

	// ping to first and its alias foo must succeed
	_, _, err := dockerCmdWithError("exec", "second", "ping", "-c", "1", "first")
	assert.NilError(c, err)
	_, _, err = dockerCmdWithError("exec", "second", "ping", "-c", "1", "foo")
	assert.NilError(c, err)

	// ping to third and its alias must fail
	_, _, err = dockerCmdWithError("exec", "second", "ping", "-c", "1", "third")
	assert.ErrorContains(c, err, "")
	_, _, err = dockerCmdWithError("exec", "second", "ping", "-c", "1", "bar")
	assert.ErrorContains(c, err, "")

	// start third container now
	dockerCmd(c, "run", "-d", "--net=udlinkNet", "--name=third", "busybox", "top")
	assert.Assert(c, waitRun("third") == nil)

	// ping to third and its alias must succeed now
	_, _, err = dockerCmdWithError("exec", "second", "ping", "-c", "1", "third")
	assert.NilError(c, err)
	_, _, err = dockerCmdWithError("exec", "second", "ping", "-c", "1", "bar")
	assert.NilError(c, err)
}

func (s *DockerSuite) TestUserDefinedNetworkLinksWithRestart(c *testing.T) {
	testRequires(c, DaemonIsLinux, NotUserNamespace, NotArm)
	dockerCmd(c, "network", "create", "-d", "bridge", "udlinkNet")

	dockerCmd(c, "run", "-d", "--net=udlinkNet", "--name=first", "busybox", "top")
	assert.Assert(c, waitRun("first") == nil)

	dockerCmd(c, "run", "-d", "--net=udlinkNet", "--name=second", "--link=first:foo",
		"busybox", "top")
	assert.Assert(c, waitRun("second") == nil)

	// ping to first and its alias foo must succeed
	_, _, err := dockerCmdWithError("exec", "second", "ping", "-c", "1", "first")
	assert.NilError(c, err)
	_, _, err = dockerCmdWithError("exec", "second", "ping", "-c", "1", "foo")
	assert.NilError(c, err)

	// Restart first container
	dockerCmd(c, "restart", "first")
	assert.Assert(c, waitRun("first") == nil)

	// ping to first and its alias foo must still succeed
	_, _, err = dockerCmdWithError("exec", "second", "ping", "-c", "1", "first")
	assert.NilError(c, err)
	_, _, err = dockerCmdWithError("exec", "second", "ping", "-c", "1", "foo")
	assert.NilError(c, err)

	// Restart second container
	dockerCmd(c, "restart", "second")
	assert.Assert(c, waitRun("second") == nil)

	// ping to first and its alias foo must still succeed
	_, _, err = dockerCmdWithError("exec", "second", "ping", "-c", "1", "first")
	assert.NilError(c, err)
	_, _, err = dockerCmdWithError("exec", "second", "ping", "-c", "1", "foo")
	assert.NilError(c, err)
}

func (s *DockerSuite) TestRunWithNetAliasOnDefaultNetworks(c *testing.T) {
	testRequires(c, DaemonIsLinux, NotUserNamespace, NotArm)

	defaults := []string{"bridge", "host", "none"}
	for _, net := range defaults {
		out, _, err := dockerCmdWithError("run", "-d", "--net", net, "--net-alias", "alias_"+net, "busybox", "top")
		assert.ErrorContains(c, err, "")
		assert.Assert(c, strings.Contains(out, runconfig.ErrUnsupportedNetworkAndAlias.Error()))
	}
}

func (s *DockerSuite) TestUserDefinedNetworkAlias(c *testing.T) {
	testRequires(c, DaemonIsLinux, NotUserNamespace, NotArm)
	dockerCmd(c, "network", "create", "-d", "bridge", "net1")

	cid1, _ := dockerCmd(c, "run", "-d", "--net=net1", "--name=first", "--net-alias=foo1", "--net-alias=foo2", "busybox:glibc", "top")
	assert.Assert(c, waitRun("first") == nil)

	// Check if default short-id alias is added automatically
	id := strings.TrimSpace(cid1)
	aliases := inspectField(c, id, "NetworkSettings.Networks.net1.Aliases")
	assert.Assert(c, strings.Contains(aliases, stringid.TruncateID(id)))
	cid2, _ := dockerCmd(c, "run", "-d", "--net=net1", "--name=second", "busybox:glibc", "top")
	assert.Assert(c, waitRun("second") == nil)

	// Check if default short-id alias is added automatically
	id = strings.TrimSpace(cid2)
	aliases = inspectField(c, id, "NetworkSettings.Networks.net1.Aliases")
	assert.Assert(c, strings.Contains(aliases, stringid.TruncateID(id)))
	// ping to first and its network-scoped aliases
	_, _, err := dockerCmdWithError("exec", "second", "ping", "-c", "1", "first")
	assert.NilError(c, err)
	_, _, err = dockerCmdWithError("exec", "second", "ping", "-c", "1", "foo1")
	assert.NilError(c, err)
	_, _, err = dockerCmdWithError("exec", "second", "ping", "-c", "1", "foo2")
	assert.NilError(c, err)
	// ping first container's short-id alias
	_, _, err = dockerCmdWithError("exec", "second", "ping", "-c", "1", stringid.TruncateID(cid1))
	assert.NilError(c, err)

	// Restart first container
	dockerCmd(c, "restart", "first")
	assert.Assert(c, waitRun("first") == nil)

	// ping to first and its network-scoped aliases must succeed
	_, _, err = dockerCmdWithError("exec", "second", "ping", "-c", "1", "first")
	assert.NilError(c, err)
	_, _, err = dockerCmdWithError("exec", "second", "ping", "-c", "1", "foo1")
	assert.NilError(c, err)
	_, _, err = dockerCmdWithError("exec", "second", "ping", "-c", "1", "foo2")
	assert.NilError(c, err)
	// ping first container's short-id alias
	_, _, err = dockerCmdWithError("exec", "second", "ping", "-c", "1", stringid.TruncateID(cid1))
	assert.NilError(c, err)
}

// Issue 9677.
func (s *DockerSuite) TestRunWithDaemonFlags(c *testing.T) {
	out, _, err := dockerCmdWithError("--exec-opt", "foo=bar", "run", "-i", "busybox", "true")
	assert.ErrorContains(c, err, "")
	assert.Assert(c, strings.Contains(out, "unknown flag: --exec-opt"))
}

// Regression test for #4979
func (s *DockerSuite) TestRunWithVolumesFromExited(c *testing.T) {

	var (
		out      string
		exitCode int
	)

	// Create a file in a volume
	if testEnv.OSType == "windows" {
		out, exitCode = dockerCmd(c, "run", "--name", "test-data", "--volume", `c:\some\dir`, testEnv.PlatformDefaults.BaseImage, "cmd", "/c", `echo hello > c:\some\dir\file`)
	} else {
		out, exitCode = dockerCmd(c, "run", "--name", "test-data", "--volume", "/some/dir", "busybox", "touch", "/some/dir/file")
	}
	if exitCode != 0 {
		c.Fatal("1", out, exitCode)
	}

	// Read the file from another container using --volumes-from to access the volume in the second container
	if testEnv.OSType == "windows" {
		out, exitCode = dockerCmd(c, "run", "--volumes-from", "test-data", testEnv.PlatformDefaults.BaseImage, "cmd", "/c", `type c:\some\dir\file`)
	} else {
		out, exitCode = dockerCmd(c, "run", "--volumes-from", "test-data", "busybox", "cat", "/some/dir/file")
	}
	if exitCode != 0 {
		c.Fatal("2", out, exitCode)
	}
}

// Volume path is a symlink which also exists on the host, and the host side is a file not a dir
// But the volume call is just a normal volume, not a bind mount
func (s *DockerSuite) TestRunCreateVolumesInSymlinkDir(c *testing.T) {
	var (
		dockerFile    string
		containerPath string
		cmd           string
	)
	// This test cannot run on a Windows daemon as
	// Windows does not support symlinks inside a volume path
	testRequires(c, testEnv.IsLocalDaemon, DaemonIsLinux)
	name := "test-volume-symlink"

	dir, err := ioutil.TempDir("", name)
	if err != nil {
		c.Fatal(err)
	}
	defer os.RemoveAll(dir)

	// In the case of Windows to Windows CI, if the machine is setup so that
	// the temp directory is not the C: drive, this test is invalid and will
	// not work.
	if testEnv.OSType == "windows" && strings.ToLower(dir[:1]) != "c" {
		c.Skip("Requires TEMP to point to C: drive")
	}

	f, err := os.OpenFile(filepath.Join(dir, "test"), os.O_CREATE, 0700)
	if err != nil {
		c.Fatal(err)
	}
	f.Close()

	if testEnv.OSType == "windows" {
		dockerFile = fmt.Sprintf("FROM %s\nRUN mkdir %s\nRUN mklink /D c:\\test %s", testEnv.PlatformDefaults.BaseImage, dir, dir)
		containerPath = `c:\test\test`
		cmd = "tasklist"
	} else {
		dockerFile = fmt.Sprintf("FROM busybox\nRUN mkdir -p %s\nRUN ln -s %s /test", dir, dir)
		containerPath = "/test/test"
		cmd = "true"
	}
	buildImageSuccessfully(c, name, build.WithDockerfile(dockerFile))
	dockerCmd(c, "run", "-v", containerPath, name, cmd)
}

// Volume path is a symlink in the container
func (s *DockerSuite) TestRunCreateVolumesInSymlinkDir2(c *testing.T) {
	var (
		dockerFile    string
		containerPath string
		cmd           string
	)
	// This test cannot run on a Windows daemon as
	// Windows does not support symlinks inside a volume path
	testRequires(c, testEnv.IsLocalDaemon, DaemonIsLinux)
	name := "test-volume-symlink2"

	if testEnv.OSType == "windows" {
		dockerFile = fmt.Sprintf("FROM %s\nRUN mkdir c:\\%s\nRUN mklink /D c:\\test c:\\%s", testEnv.PlatformDefaults.BaseImage, name, name)
		containerPath = `c:\test\test`
		cmd = "tasklist"
	} else {
		dockerFile = fmt.Sprintf("FROM busybox\nRUN mkdir -p /%s\nRUN ln -s /%s /test", name, name)
		containerPath = "/test/test"
		cmd = "true"
	}
	buildImageSuccessfully(c, name, build.WithDockerfile(dockerFile))
	dockerCmd(c, "run", "-v", containerPath, name, cmd)
}

func (s *DockerSuite) TestRunVolumesMountedAsReadonly(c *testing.T) {
	if _, code, err := dockerCmdWithError("run", "-v", "/test:/test:ro", "busybox", "touch", "/test/somefile"); err == nil || code == 0 {
		c.Fatalf("run should fail because volume is ro: exit code %d", code)
	}
}

func (s *DockerSuite) TestRunVolumesFromInReadonlyModeFails(c *testing.T) {
	var (
		volumeDir string
		fileInVol string
	)
	if testEnv.OSType == "windows" {
		volumeDir = `c:/test` // Forward-slash as using busybox
		fileInVol = `c:/test/file`
	} else {
		testRequires(c, DaemonIsLinux)
		volumeDir = "/test"
		fileInVol = `/test/file`
	}
	dockerCmd(c, "run", "--name", "parent", "-v", volumeDir, "busybox", "true")

	if _, code, err := dockerCmdWithError("run", "--volumes-from", "parent:ro", "busybox", "touch", fileInVol); err == nil || code == 0 {
		c.Fatalf("run should fail because volume is ro: exit code %d", code)
	}
}

// Regression test for #1201
func (s *DockerSuite) TestRunVolumesFromInReadWriteMode(c *testing.T) {
	var (
		volumeDir string
		fileInVol string
	)
	if testEnv.OSType == "windows" {
		volumeDir = `c:/test` // Forward-slash as using busybox
		fileInVol = `c:/test/file`
	} else {
		volumeDir = "/test"
		fileInVol = "/test/file"
	}

	dockerCmd(c, "run", "--name", "parent", "-v", volumeDir, "busybox", "true")
	dockerCmd(c, "run", "--volumes-from", "parent:rw", "busybox", "touch", fileInVol)

	if out, _, err := dockerCmdWithError("run", "--volumes-from", "parent:bar", "busybox", "touch", fileInVol); err == nil || !strings.Contains(out, `invalid mode: bar`) {
		c.Fatalf("running --volumes-from parent:bar should have failed with invalid mode: %q", out)
	}

	dockerCmd(c, "run", "--volumes-from", "parent", "busybox", "touch", fileInVol)
}

func (s *DockerSuite) TestVolumesFromGetsProperMode(c *testing.T) {
	testRequires(c, testEnv.IsLocalDaemon)
	prefix, slash := getPrefixAndSlashFromDaemonPlatform()
	hostpath := RandomTmpDirPath("test", testEnv.OSType)
	if err := os.MkdirAll(hostpath, 0755); err != nil {
		c.Fatalf("Failed to create %s: %q", hostpath, err)
	}
	defer os.RemoveAll(hostpath)

	dockerCmd(c, "run", "--name", "parent", "-v", hostpath+":"+prefix+slash+"test:ro", "busybox", "true")

	// Expect this "rw" mode to be be ignored since the inherited volume is "ro"
	if _, _, err := dockerCmdWithError("run", "--volumes-from", "parent:rw", "busybox", "touch", prefix+slash+"test"+slash+"file"); err == nil {
		c.Fatal("Expected volumes-from to inherit read-only volume even when passing in `rw`")
	}

	dockerCmd(c, "run", "--name", "parent2", "-v", hostpath+":"+prefix+slash+"test:ro", "busybox", "true")

	// Expect this to be read-only since both are "ro"
	if _, _, err := dockerCmdWithError("run", "--volumes-from", "parent2:ro", "busybox", "touch", prefix+slash+"test"+slash+"file"); err == nil {
		c.Fatal("Expected volumes-from to inherit read-only volume even when passing in `ro`")
	}
}

// Test for GH#10618
func (s *DockerSuite) TestRunNoDupVolumes(c *testing.T) {
	path1 := RandomTmpDirPath("test1", testEnv.OSType)
	path2 := RandomTmpDirPath("test2", testEnv.OSType)

	someplace := ":/someplace"
	if testEnv.OSType == "windows" {
		// Windows requires that the source directory exists before calling HCS
		testRequires(c, testEnv.IsLocalDaemon)
		someplace = `:c:\someplace`
		if err := os.MkdirAll(path1, 0755); err != nil {
			c.Fatalf("Failed to create %s: %q", path1, err)
		}
		defer os.RemoveAll(path1)
		if err := os.MkdirAll(path2, 0755); err != nil {
			c.Fatalf("Failed to create %s: %q", path1, err)
		}
		defer os.RemoveAll(path2)
	}
	mountstr1 := path1 + someplace
	mountstr2 := path2 + someplace

	if out, _, err := dockerCmdWithError("run", "-v", mountstr1, "-v", mountstr2, "busybox", "true"); err == nil {
		c.Fatal("Expected error about duplicate mount definitions")
	} else {
		if !strings.Contains(out, "Duplicate mount point") {
			c.Fatalf("Expected 'duplicate mount point' error, got %v", out)
		}
	}

	// Test for https://github.com/docker/docker/issues/22093
	volumename1 := "test1"
	volumename2 := "test2"
	volume1 := volumename1 + someplace
	volume2 := volumename2 + someplace
	if out, _, err := dockerCmdWithError("run", "-v", volume1, "-v", volume2, "busybox", "true"); err == nil {
		c.Fatal("Expected error about duplicate mount definitions")
	} else {
		if !strings.Contains(out, "Duplicate mount point") {
			c.Fatalf("Expected 'duplicate mount point' error, got %v", out)
		}
	}
	// create failed should have create volume volumename1 or volumename2
	// we should remove volumename2 or volumename2 successfully
	out, _ := dockerCmd(c, "volume", "ls")
	if strings.Contains(out, volumename1) {
		dockerCmd(c, "volume", "rm", volumename1)
	} else {
		dockerCmd(c, "volume", "rm", volumename2)
	}
}

// Test for #1351
func (s *DockerSuite) TestRunApplyVolumesFromBeforeVolumes(c *testing.T) {
	prefix := ""
	if testEnv.OSType == "windows" {
		prefix = `c:`
	}
	dockerCmd(c, "run", "--name", "parent", "-v", prefix+"/test", "busybox", "touch", prefix+"/test/foo")
	dockerCmd(c, "run", "--volumes-from", "parent", "-v", prefix+"/test", "busybox", "cat", prefix+"/test/foo")
}

func (s *DockerSuite) TestRunMultipleVolumesFrom(c *testing.T) {
	prefix := ""
	if testEnv.OSType == "windows" {
		prefix = `c:`
	}
	dockerCmd(c, "run", "--name", "parent1", "-v", prefix+"/test", "busybox", "touch", prefix+"/test/foo")
	dockerCmd(c, "run", "--name", "parent2", "-v", prefix+"/other", "busybox", "touch", prefix+"/other/bar")
	dockerCmd(c, "run", "--volumes-from", "parent1", "--volumes-from", "parent2", "busybox", "sh", "-c", "cat /test/foo && cat /other/bar")
}

// this tests verifies the ID format for the container
func (s *DockerSuite) TestRunVerifyContainerID(c *testing.T) {
	out, exit, err := dockerCmdWithError("run", "-d", "busybox", "true")
	if err != nil {
		c.Fatal(err)
	}
	if exit != 0 {
		c.Fatalf("expected exit code 0 received %d", exit)
	}

	match, err := regexp.MatchString("^[0-9a-f]{64}$", strings.TrimSuffix(out, "\n"))
	if err != nil {
		c.Fatal(err)
	}
	if !match {
		c.Fatalf("Invalid container ID: %s", out)
	}
}

// Test that creating a container with a volume doesn't crash. Regression test for #995.
func (s *DockerSuite) TestRunCreateVolume(c *testing.T) {
	prefix := ""
	if testEnv.OSType == "windows" {
		prefix = `c:`
	}
	dockerCmd(c, "run", "-v", prefix+"/var/lib/data", "busybox", "true")
}

// Test that creating a volume with a symlink in its path works correctly. Test for #5152.
// Note that this bug happens only with symlinks with a target that starts with '/'.
func (s *DockerSuite) TestRunCreateVolumeWithSymlink(c *testing.T) {
	// Cannot run on Windows as relies on Linux-specific functionality (sh -c mount...)
	testRequires(c, DaemonIsLinux)
	workingDirectory, err := ioutil.TempDir("", "TestRunCreateVolumeWithSymlink")
	assert.NilError(c, err)
	image := "docker-test-createvolumewithsymlink"

	buildCmd := exec.Command(dockerBinary, "build", "-t", image, "-")
	buildCmd.Stdin = strings.NewReader(`FROM busybox
		RUN ln -s home /bar`)
	buildCmd.Dir = workingDirectory
	err = buildCmd.Run()
	if err != nil {
		c.Fatalf("could not build '%s': %v", image, err)
	}

	_, exitCode, err := dockerCmdWithError("run", "-v", "/bar/foo", "--name", "test-createvolumewithsymlink", image, "sh", "-c", "mount | grep -q /home/foo")
	if err != nil || exitCode != 0 {
		c.Fatalf("[run] err: %v, exitcode: %d", err, exitCode)
	}

	volPath, err := inspectMountSourceField("test-createvolumewithsymlink", "/bar/foo")
	assert.NilError(c, err)

	_, exitCode, err = dockerCmdWithError("rm", "-v", "test-createvolumewithsymlink")
	if err != nil || exitCode != 0 {
		c.Fatalf("[rm] err: %v, exitcode: %d", err, exitCode)
	}

	_, err = os.Stat(volPath)
	if !os.IsNotExist(err) {
		c.Fatalf("[open] (expecting 'file does not exist' error) err: %v, volPath: %s", err, volPath)
	}
}

// Tests that a volume path that has a symlink exists in a container mounting it with `--volumes-from`.
func (s *DockerSuite) TestRunVolumesFromSymlinkPath(c *testing.T) {
	// This test cannot run on a Windows daemon as
	// Windows does not support symlinks inside a volume path
	testRequires(c, DaemonIsLinux)

	workingDirectory, err := ioutil.TempDir("", "TestRunVolumesFromSymlinkPath")
	assert.NilError(c, err)
	name := "docker-test-volumesfromsymlinkpath"
	prefix := ""
	dfContents := `FROM busybox
		RUN ln -s home /foo
		VOLUME ["/foo/bar"]`

	if testEnv.OSType == "windows" {
		prefix = `c:`
		dfContents = `FROM ` + testEnv.PlatformDefaults.BaseImage + `
	    RUN mkdir c:\home
		RUN mklink /D c:\foo c:\home
		VOLUME ["c:/foo/bar"]
		ENTRYPOINT c:\windows\system32\cmd.exe`
	}

	buildCmd := exec.Command(dockerBinary, "build", "-t", name, "-")
	buildCmd.Stdin = strings.NewReader(dfContents)
	buildCmd.Dir = workingDirectory
	err = buildCmd.Run()
	if err != nil {
		c.Fatalf("could not build 'docker-test-volumesfromsymlinkpath': %v", err)
	}

	out, exitCode, err := dockerCmdWithError("run", "--name", "test-volumesfromsymlinkpath", name)
	if err != nil || exitCode != 0 {
		c.Fatalf("[run] (volume) err: %v, exitcode: %d, out: %s", err, exitCode, out)
	}

	_, exitCode, err = dockerCmdWithError("run", "--volumes-from", "test-volumesfromsymlinkpath", "busybox", "sh", "-c", "ls "+prefix+"/foo | grep -q bar")
	if err != nil || exitCode != 0 {
		c.Fatalf("[run] err: %v, exitcode: %d", err, exitCode)
	}
}

func (s *DockerSuite) TestRunExitCode(c *testing.T) {
	var (
		exit int
		err  error
	)

	_, exit, err = dockerCmdWithError("run", "busybox", "/bin/sh", "-c", "exit 72")

	if err == nil {
		c.Fatal("should not have a non nil error")
	}
	if exit != 72 {
		c.Fatalf("expected exit code 72 received %d", exit)
	}
}

func (s *DockerSuite) TestRunUserDefaults(c *testing.T) {
	expected := "uid=0(root) gid=0(root)"
	if testEnv.OSType == "windows" {
		expected = "uid=0(root) gid=0(root) groups=0(root)"
	}
	out, _ := dockerCmd(c, "run", "busybox", "id")
	if !strings.Contains(out, expected) {
		c.Fatalf("expected '%s' got %s", expected, out)
	}
}

func (s *DockerSuite) TestRunUserByName(c *testing.T) {
	// TODO Windows: This test cannot run on a Windows daemon as Windows does
	// not support the use of -u
	testRequires(c, DaemonIsLinux)
	out, _ := dockerCmd(c, "run", "-u", "root", "busybox", "id")
	if !strings.Contains(out, "uid=0(root) gid=0(root)") {
		c.Fatalf("expected root user got %s", out)
	}
}

func (s *DockerSuite) TestRunUserByID(c *testing.T) {
	// TODO Windows: This test cannot run on a Windows daemon as Windows does
	// not support the use of -u
	testRequires(c, DaemonIsLinux)
	out, _ := dockerCmd(c, "run", "-u", "1", "busybox", "id")
	if !strings.Contains(out, "uid=1(daemon) gid=1(daemon)") {
		c.Fatalf("expected daemon user got %s", out)
	}
}

func (s *DockerSuite) TestRunUserByIDBig(c *testing.T) {
	// TODO Windows: This test cannot run on a Windows daemon as Windows does
	// not support the use of -u
	testRequires(c, DaemonIsLinux, NotArm)
	out, _, err := dockerCmdWithError("run", "-u", "2147483648", "busybox", "id")
	if err == nil {
		c.Fatal("No error, but must be.", out)
	}
	if !strings.Contains(strings.ToLower(out), "uids and gids must be in range") {
		c.Fatalf("expected error about uids range, got %s", out)
	}
}

func (s *DockerSuite) TestRunUserByIDNegative(c *testing.T) {
	// TODO Windows: This test cannot run on a Windows daemon as Windows does
	// not support the use of -u
	testRequires(c, DaemonIsLinux)
	out, _, err := dockerCmdWithError("run", "-u", "-1", "busybox", "id")
	if err == nil {
		c.Fatal("No error, but must be.", out)
	}
	if !strings.Contains(strings.ToLower(out), "uids and gids must be in range") {
		c.Fatalf("expected error about uids range, got %s", out)
	}
}

func (s *DockerSuite) TestRunUserByIDZero(c *testing.T) {
	// TODO Windows: This test cannot run on a Windows daemon as Windows does
	// not support the use of -u
	testRequires(c, DaemonIsLinux)
	out, _, err := dockerCmdWithError("run", "-u", "0", "busybox", "id")
	if err != nil {
		c.Fatal(err, out)
	}
	if !strings.Contains(out, "uid=0(root) gid=0(root) groups=10(wheel)") {
		c.Fatalf("expected daemon user got %s", out)
	}
}

func (s *DockerSuite) TestRunUserNotFound(c *testing.T) {
	// TODO Windows: This test cannot run on a Windows daemon as Windows does
	// not support the use of -u
	testRequires(c, DaemonIsLinux)
	_, _, err := dockerCmdWithError("run", "-u", "notme", "busybox", "id")
	if err == nil {
		c.Fatal("unknown user should cause container to fail")
	}
}

func (s *DockerSuite) TestRunTwoConcurrentContainers(c *testing.T) {
	sleepTime := "2"
	group := sync.WaitGroup{}
	group.Add(2)

	errChan := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func() {
			defer group.Done()
			_, _, err := dockerCmdWithError("run", "busybox", "sleep", sleepTime)
			errChan <- err
		}()
	}

	group.Wait()
	close(errChan)

	for err := range errChan {
		assert.NilError(c, err)
	}
}

func (s *DockerSuite) TestRunEnvironment(c *testing.T) {
	// TODO Windows: Environment handling is different between Linux and
	// Windows and this test relies currently on unix functionality.
	testRequires(c, DaemonIsLinux)
	result := icmd.RunCmd(icmd.Cmd{
		Command: []string{dockerBinary, "run", "-h", "testing", "-e=FALSE=true", "-e=TRUE", "-e=TRICKY", "-e=HOME=", "busybox", "env"},
		Env: append(os.Environ(),
			"TRUE=false",
			"TRICKY=tri\ncky\n",
		),
	})
	result.Assert(c, icmd.Success)

	actualEnv := strings.Split(strings.TrimSuffix(result.Stdout(), "\n"), "\n")
	sort.Strings(actualEnv)

	goodEnv := []string{
		// The first two should not be tested here, those are "inherent" environment variable. This test validates
		// the -e behavior, not the default environment variable (that could be subject to change)
		"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
		"HOSTNAME=testing",
		"FALSE=true",
		"TRUE=false",
		"TRICKY=tri",
		"cky",
		"",
		"HOME=/root",
	}
	sort.Strings(goodEnv)
	if len(goodEnv) != len(actualEnv) {
		c.Fatalf("Wrong environment: should be %d variables, not %d: %q", len(goodEnv), len(actualEnv), strings.Join(actualEnv, ", "))
	}
	for i := range goodEnv {
		if actualEnv[i] != goodEnv[i] {
			c.Fatalf("Wrong environment variable: should be %s, not %s", goodEnv[i], actualEnv[i])
		}
	}
}

func (s *DockerSuite) TestRunEnvironmentErase(c *testing.T) {
	// TODO Windows: Environment handling is different between Linux and
	// Windows and this test relies currently on unix functionality.
	testRequires(c, DaemonIsLinux)

	// Test to make sure that when we use -e on env vars that are
	// not set in our local env that they're removed (if present) in
	// the container

	result := icmd.RunCmd(icmd.Cmd{
		Command: []string{dockerBinary, "run", "-e", "FOO", "-e", "HOSTNAME", "busybox", "env"},
		Env:     appendBaseEnv(true),
	})
	result.Assert(c, icmd.Success)

	actualEnv := strings.Split(strings.TrimSpace(result.Combined()), "\n")
	sort.Strings(actualEnv)

	goodEnv := []string{
		"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
		"HOME=/root",
	}
	sort.Strings(goodEnv)
	if len(goodEnv) != len(actualEnv) {
		c.Fatalf("Wrong environment: should be %d variables, not %d: %q", len(goodEnv), len(actualEnv), strings.Join(actualEnv, ", "))
	}
	for i := range goodEnv {
		if actualEnv[i] != goodEnv[i] {
			c.Fatalf("Wrong environment variable: should be %s, not %s", goodEnv[i], actualEnv[i])
		}
	}
}

func (s *DockerSuite) TestRunEnvironmentOverride(c *testing.T) {
	// TODO Windows: Environment handling is different between Linux and
	// Windows and this test relies currently on unix functionality.
	testRequires(c, DaemonIsLinux)

	// Test to make sure that when we use -e on env vars that are
	// already in the env that we're overriding them

	result := icmd.RunCmd(icmd.Cmd{
		Command: []string{dockerBinary, "run", "-e", "HOSTNAME", "-e", "HOME=/root2", "busybox", "env"},
		Env:     appendBaseEnv(true, "HOSTNAME=bar"),
	})
	result.Assert(c, icmd.Success)

	actualEnv := strings.Split(strings.TrimSpace(result.Combined()), "\n")
	sort.Strings(actualEnv)

	goodEnv := []string{
		"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
		"HOME=/root2",
		"HOSTNAME=bar",
	}
	sort.Strings(goodEnv)
	if len(goodEnv) != len(actualEnv) {
		c.Fatalf("Wrong environment: should be %d variables, not %d: %q", len(goodEnv), len(actualEnv), strings.Join(actualEnv, ", "))
	}
	for i := range goodEnv {
		if actualEnv[i] != goodEnv[i] {
			c.Fatalf("Wrong environment variable: should be %s, not %s", goodEnv[i], actualEnv[i])
		}
	}
}

func (s *DockerSuite) TestRunContainerNetwork(c *testing.T) {
	if testEnv.OSType == "windows" {
		// Windows busybox does not have ping. Use built in ping instead.
		dockerCmd(c, "run", testEnv.PlatformDefaults.BaseImage, "ping", "-n", "1", "127.0.0.1")
	} else {
		dockerCmd(c, "run", "busybox", "ping", "-c", "1", "127.0.0.1")
	}
}

func (s *DockerSuite) TestRunNetHostNotAllowedWithLinks(c *testing.T) {
	// TODO Windows: This is Linux specific as --link is not supported and
	// this will be deprecated in favor of container networking model.
	testRequires(c, DaemonIsLinux, NotUserNamespace)
	dockerCmd(c, "run", "--name", "linked", "busybox", "true")

	_, _, err := dockerCmdWithError("run", "--net=host", "--link", "linked:linked", "busybox", "true")
	if err == nil {
		c.Fatal("Expected error")
	}
}

// #7851 hostname outside container shows FQDN, inside only shortname
// For testing purposes it is not required to set host's hostname directly
// and use "--net=host" (as the original issue submitter did), as the same
// codepath is executed with "docker run -h <hostname>".  Both were manually
// tested, but this testcase takes the simpler path of using "run -h .."
func (s *DockerSuite) TestRunFullHostnameSet(c *testing.T) {
	// TODO Windows: -h is not yet functional.
	testRequires(c, DaemonIsLinux)
	out, _ := dockerCmd(c, "run", "-h", "foo.bar.baz", "busybox", "hostname")
	if actual := strings.Trim(out, "\r\n"); actual != "foo.bar.baz" {
		c.Fatalf("expected hostname 'foo.bar.baz', received %s", actual)
	}
}

func (s *DockerSuite) TestRunPrivilegedCanMknod(c *testing.T) {
	// Not applicable for Windows as Windows daemon does not support
	// the concept of --privileged, and mknod is a Unix concept.
	testRequires(c, DaemonIsLinux, NotUserNamespace)
	out, _ := dockerCmd(c, "run", "--privileged", "busybox", "sh", "-c", "mknod /tmp/sda b 8 0 && echo ok")
	if actual := strings.Trim(out, "\r\n"); actual != "ok" {
		c.Fatalf("expected output ok received %s", actual)
	}
}

func (s *DockerSuite) TestRunUnprivilegedCanMknod(c *testing.T) {
	// Not applicable for Windows as Windows daemon does not support
	// the concept of --privileged, and mknod is a Unix concept.
	testRequires(c, DaemonIsLinux, NotUserNamespace)
	out, _ := dockerCmd(c, "run", "busybox", "sh", "-c", "mknod /tmp/sda b 8 0 && echo ok")
	if actual := strings.Trim(out, "\r\n"); actual != "ok" {
		c.Fatalf("expected output ok received %s", actual)
	}
}

func (s *DockerSuite) TestRunCapDropInvalid(c *testing.T) {
	// Not applicable for Windows as there is no concept of --cap-drop
	testRequires(c, DaemonIsLinux)
	out, _, err := dockerCmdWithError("run", "--cap-drop=CHPASS", "busybox", "ls")
	if err == nil {
		c.Fatal(err, out)
	}
}

func (s *DockerSuite) TestRunCapDropCannotMknod(c *testing.T) {
	// Not applicable for Windows as there is no concept of --cap-drop or mknod
	testRequires(c, DaemonIsLinux)
	out, _, err := dockerCmdWithError("run", "--cap-drop=MKNOD", "busybox", "sh", "-c", "mknod /tmp/sda b 8 0 && echo ok")

	if err == nil {
		c.Fatal(err, out)
	}
	if actual := strings.Trim(out, "\r\n"); actual == "ok" {
		c.Fatalf("expected output not ok received %s", actual)
	}
}

func (s *DockerSuite) TestRunCapDropCannotMknodLowerCase(c *testing.T) {
	// Not applicable for Windows as there is no concept of --cap-drop or mknod
	testRequires(c, DaemonIsLinux)
	out, _, err := dockerCmdWithError("run", "--cap-drop=mknod", "busybox", "sh", "-c", "mknod /tmp/sda b 8 0 && echo ok")

	if err == nil {
		c.Fatal(err, out)
	}
	if actual := strings.Trim(out, "\r\n"); actual == "ok" {
		c.Fatalf("expected output not ok received %s", actual)
	}
}

func (s *DockerSuite) TestRunCapDropALLCannotMknod(c *testing.T) {
	// Not applicable for Windows as there is no concept of --cap-drop or mknod
	testRequires(c, DaemonIsLinux)
	out, _, err := dockerCmdWithError("run", "--cap-drop=ALL", "--cap-add=SETGID", "busybox", "sh", "-c", "mknod /tmp/sda b 8 0 && echo ok")
	if err == nil {
		c.Fatal(err, out)
	}
	if actual := strings.Trim(out, "\r\n"); actual == "ok" {
		c.Fatalf("expected output not ok received %s", actual)
	}
}

func (s *DockerSuite) TestRunCapDropALLAddMknodCanMknod(c *testing.T) {
	// Not applicable for Windows as there is no concept of --cap-drop or mknod
	testRequires(c, DaemonIsLinux, NotUserNamespace)
	out, _ := dockerCmd(c, "run", "--cap-drop=ALL", "--cap-add=MKNOD", "--cap-add=SETGID", "busybox", "sh", "-c", "mknod /tmp/sda b 8 0 && echo ok")

	if actual := strings.Trim(out, "\r\n"); actual != "ok" {
		c.Fatalf("expected output ok received %s", actual)
	}
}

func (s *DockerSuite) TestRunCapAddInvalid(c *testing.T) {
	// Not applicable for Windows as there is no concept of --cap-add
	testRequires(c, DaemonIsLinux)
	out, _, err := dockerCmdWithError("run", "--cap-add=CHPASS", "busybox", "ls")
	if err == nil {
		c.Fatal(err, out)
	}
}

func (s *DockerSuite) TestRunCapAddCanDownInterface(c *testing.T) {
	// Not applicable for Windows as there is no concept of --cap-add
	testRequires(c, DaemonIsLinux)
	out, _ := dockerCmd(c, "run", "--cap-add=NET_ADMIN", "busybox", "sh", "-c", "ip link set eth0 down && echo ok")

	if actual := strings.Trim(out, "\r\n"); actual != "ok" {
		c.Fatalf("expected output ok received %s", actual)
	}
}

func (s *DockerSuite) TestRunCapAddALLCanDownInterface(c *testing.T) {
	// Not applicable for Windows as there is no concept of --cap-add
	testRequires(c, DaemonIsLinux)
	out, _ := dockerCmd(c, "run", "--cap-add=ALL", "busybox", "sh", "-c", "ip link set eth0 down && echo ok")

	if actual := strings.Trim(out, "\r\n"); actual != "ok" {
		c.Fatalf("expected output ok received %s", actual)
	}
}

func (s *DockerSuite) TestRunCapAddALLDropNetAdminCanDownInterface(c *testing.T) {
	// Not applicable for Windows as there is no concept of --cap-add
	testRequires(c, DaemonIsLinux)
	out, _, err := dockerCmdWithError("run", "--cap-add=ALL", "--cap-drop=NET_ADMIN", "busybox", "sh", "-c", "ip link set eth0 down && echo ok")
	if err == nil {
		c.Fatal(err, out)
	}
	if actual := strings.Trim(out, "\r\n"); actual == "ok" {
		c.Fatalf("expected output not ok received %s", actual)
	}
}

func (s *DockerSuite) TestRunGroupAdd(c *testing.T) {
	// Not applicable for Windows as there is no concept of --group-add
	testRequires(c, DaemonIsLinux)
	out, _ := dockerCmd(c, "run", "--group-add=audio", "--group-add=staff", "--group-add=777", "busybox", "sh", "-c", "id")

	groupsList := "uid=0(root) gid=0(root) groups=10(wheel),29(audio),50(staff),777"
	if actual := strings.Trim(out, "\r\n"); actual != groupsList {
		c.Fatalf("expected output %s received %s", groupsList, actual)
	}
}

func (s *DockerSuite) TestRunPrivilegedCanMount(c *testing.T) {
	// Not applicable for Windows as there is no concept of --privileged
	testRequires(c, DaemonIsLinux, NotUserNamespace)
	out, _ := dockerCmd(c, "run", "--privileged", "busybox", "sh", "-c", "mount -t tmpfs none /tmp && echo ok")

	if actual := strings.Trim(out, "\r\n"); actual != "ok" {
		c.Fatalf("expected output ok received %s", actual)
	}
}

func (s *DockerSuite) TestRunUnprivilegedCannotMount(c *testing.T) {
	// Not applicable for Windows as there is no concept of unprivileged
	testRequires(c, DaemonIsLinux)
	out, _, err := dockerCmdWithError("run", "busybox", "sh", "-c", "mount -t tmpfs none /tmp && echo ok")

	if err == nil {
		c.Fatal(err, out)
	}
	if actual := strings.Trim(out, "\r\n"); actual == "ok" {
		c.Fatalf("expected output not ok received %s", actual)
	}
}

func (s *DockerSuite) TestRunSysNotWritableInNonPrivilegedContainers(c *testing.T) {
	// Not applicable for Windows as there is no concept of unprivileged
	testRequires(c, DaemonIsLinux, NotArm)
	if _, code, err := dockerCmdWithError("run", "busybox", "touch", "/sys/kernel/profiling"); err == nil || code == 0 {
		c.Fatal("sys should not be writable in a non privileged container")
	}
}

func (s *DockerSuite) TestRunSysWritableInPrivilegedContainers(c *testing.T) {
	// Not applicable for Windows as there is no concept of unprivileged
	testRequires(c, DaemonIsLinux, NotUserNamespace, NotArm)
	if _, code, err := dockerCmdWithError("run", "--privileged", "busybox", "touch", "/sys/kernel/profiling"); err != nil || code != 0 {
		c.Fatalf("sys should be writable in privileged container")
	}
}

func (s *DockerSuite) TestRunProcNotWritableInNonPrivilegedContainers(c *testing.T) {
	// Not applicable for Windows as there is no concept of unprivileged
	testRequires(c, DaemonIsLinux)
	if _, code, err := dockerCmdWithError("run", "busybox", "touch", "/proc/sysrq-trigger"); err == nil || code == 0 {
		c.Fatal("proc should not be writable in a non privileged container")
	}
}

func (s *DockerSuite) TestRunProcWritableInPrivilegedContainers(c *testing.T) {
	// Not applicable for Windows as there is no concept of --privileged
	testRequires(c, DaemonIsLinux, NotUserNamespace)
	if _, code := dockerCmd(c, "run", "--privileged", "busybox", "sh", "-c", "touch /proc/sysrq-trigger"); code != 0 {
		c.Fatalf("proc should be writable in privileged container")
	}
}

func (s *DockerSuite) TestRunDeviceNumbers(c *testing.T) {
	// Not applicable on Windows as /dev/ is a Unix specific concept
	// TODO: NotUserNamespace could be removed here if "root" "root" is replaced w user
	testRequires(c, DaemonIsLinux, NotUserNamespace)
	out, _ := dockerCmd(c, "run", "busybox", "sh", "-c", "ls -l /dev/null")
	deviceLineFields := strings.Fields(out)
	deviceLineFields[6] = ""
	deviceLineFields[7] = ""
	deviceLineFields[8] = ""
	expected := []string{"crw-rw-rw-", "1", "root", "root", "1,", "3", "", "", "", "/dev/null"}

	if !(reflect.DeepEqual(deviceLineFields, expected)) {
		c.Fatalf("expected output\ncrw-rw-rw- 1 root root 1, 3 May 24 13:29 /dev/null\n received\n %s\n", out)
	}
}

func (s *DockerSuite) TestRunThatCharacterDevicesActLikeCharacterDevices(c *testing.T) {
	// Not applicable on Windows as /dev/ is a Unix specific concept
	testRequires(c, DaemonIsLinux)
	out, _ := dockerCmd(c, "run", "busybox", "sh", "-c", "dd if=/dev/zero of=/zero bs=1k count=5 2> /dev/null ; du -h /zero")
	if actual := strings.Trim(out, "\r\n"); actual[0] == '0' {
		c.Fatalf("expected a new file called /zero to be create that is greater than 0 bytes long, but du says: %s", actual)
	}
}

func (s *DockerSuite) TestRunUnprivilegedWithChroot(c *testing.T) {
	// Not applicable on Windows as it does not support chroot
	testRequires(c, DaemonIsLinux)
	dockerCmd(c, "run", "busybox", "chroot", "/", "true")
}

func (s *DockerSuite) TestRunAddingOptionalDevices(c *testing.T) {
	// Not applicable on Windows as Windows does not support --device
	testRequires(c, DaemonIsLinux, NotUserNamespace)
	out, _ := dockerCmd(c, "run", "--device", "/dev/zero:/dev/nulo", "busybox", "sh", "-c", "ls /dev/nulo")
	if actual := strings.Trim(out, "\r\n"); actual != "/dev/nulo" {
		c.Fatalf("expected output /dev/nulo, received %s", actual)
	}
}

func (s *DockerSuite) TestRunAddingOptionalDevicesNoSrc(c *testing.T) {
	// Not applicable on Windows as Windows does not support --device
	testRequires(c, DaemonIsLinux, NotUserNamespace)
	out, _ := dockerCmd(c, "run", "--device", "/dev/zero:rw", "busybox", "sh", "-c", "ls /dev/zero")
	if actual := strings.Trim(out, "\r\n"); actual != "/dev/zero" {
		c.Fatalf("expected output /dev/zero, received %s", actual)
	}
}

func (s *DockerSuite) TestRunAddingOptionalDevicesInvalidMode(c *testing.T) {
	// Not applicable on Windows as Windows does not support --device
	testRequires(c, DaemonIsLinux, NotUserNamespace)
	_, _, err := dockerCmdWithError("run", "--device", "/dev/zero:ro", "busybox", "sh", "-c", "ls /dev/zero")
	if err == nil {
		c.Fatalf("run container with device mode ro should fail")
	}
}

func (s *DockerSuite) TestRunModeHostname(c *testing.T) {
	// Not applicable on Windows as Windows does not support -h
	testRequires(c, testEnv.IsLocalDaemon, DaemonIsLinux, NotUserNamespace)

	out, _ := dockerCmd(c, "run", "-h=testhostname", "busybox", "cat", "/etc/hostname")

	if actual := strings.Trim(out, "\r\n"); actual != "testhostname" {
		c.Fatalf("expected 'testhostname', but says: %q", actual)
	}

	out, _ = dockerCmd(c, "run", "--net=host", "busybox", "cat", "/etc/hostname")

	hostname, err := os.Hostname()
	if err != nil {
		c.Fatal(err)
	}
	if actual := strings.Trim(out, "\r\n"); actual != hostname {
		c.Fatalf("expected %q, but says: %q", hostname, actual)
	}
}

func (s *DockerSuite) TestRunRootWorkdir(c *testing.T) {
	out, _ := dockerCmd(c, "run", "--workdir", "/", "busybox", "pwd")
	expected := "/\n"
	if testEnv.OSType == "windows" {
		expected = "C:" + expected
	}
	if out != expected {
		c.Fatalf("pwd returned %q (expected %s)", s, expected)
	}
}

func (s *DockerSuite) TestRunAllowBindMountingRoot(c *testing.T) {
	if testEnv.OSType == "windows" {
		// Windows busybox will fail with Permission Denied on items such as pagefile.sys
		dockerCmd(c, "run", "-v", `c:\:c:\host`, testEnv.PlatformDefaults.BaseImage, "cmd", "-c", "dir", `c:\host`)
	} else {
		dockerCmd(c, "run", "-v", "/:/host", "busybox", "ls", "/host")
	}
}

func (s *DockerSuite) TestRunDisallowBindMountingRootToRoot(c *testing.T) {
	mount := "/:/"
	targetDir := "/host"
	if testEnv.OSType == "windows" {
		mount = `c:\:c\`
		targetDir = "c:/host" // Forward slash as using busybox
	}
	out, _, err := dockerCmdWithError("run", "-v", mount, "busybox", "ls", targetDir)
	if err == nil {
		c.Fatal(out, err)
	}
}

// Verify that a container gets default DNS when only localhost resolvers exist
func (s *DockerSuite) TestRunDNSDefaultOptions(c *testing.T) {
	// Not applicable on Windows as this is testing Unix specific functionality
	testRequires(c, testEnv.IsLocalDaemon, DaemonIsLinux)

	// preserve original resolv.conf for restoring after test
	origResolvConf, err := ioutil.ReadFile("/etc/resolv.conf")
	if os.IsNotExist(err) {
		c.Fatalf("/etc/resolv.conf does not exist")
	}
	// defer restored original conf
	defer func() {
		if err := ioutil.WriteFile("/etc/resolv.conf", origResolvConf, 0644); err != nil {
			c.Fatal(err)
		}
	}()

	// test 3 cases: standard IPv4 localhost, commented out localhost, and IPv6 localhost
	// 2 are removed from the file at container start, and the 3rd (commented out) one is ignored by
	// GetNameservers(), leading to a replacement of nameservers with the default set
	tmpResolvConf := []byte("nameserver 127.0.0.1\n#nameserver 127.0.2.1\nnameserver ::1")
	if err := ioutil.WriteFile("/etc/resolv.conf", tmpResolvConf, 0644); err != nil {
		c.Fatal(err)
	}

	actual, _ := dockerCmd(c, "run", "busybox", "cat", "/etc/resolv.conf")
	// check that the actual defaults are appended to the commented out
	// localhost resolver (which should be preserved)
	// NOTE: if we ever change the defaults from google dns, this will break
	expected := "#nameserver 127.0.2.1\n\nnameserver 8.8.8.8\nnameserver 8.8.4.4\n"
	if actual != expected {
		c.Fatalf("expected resolv.conf be: %q, but was: %q", expected, actual)
	}
}

func (s *DockerSuite) TestRunDNSOptions(c *testing.T) {
	// Not applicable on Windows as Windows does not support --dns*, or
	// the Unix-specific functionality of resolv.conf.
	testRequires(c, DaemonIsLinux)
	result := cli.DockerCmd(c, "run", "--dns=127.0.0.1", "--dns-search=mydomain", "--dns-opt=ndots:9", "busybox", "cat", "/etc/resolv.conf")

	// The client will get a warning on stderr when setting DNS to a localhost address; verify this:
	if !strings.Contains(result.Stderr(), "Localhost DNS setting") {
		c.Fatalf("Expected warning on stderr about localhost resolver, but got %q", result.Stderr())
	}

	actual := strings.Replace(strings.Trim(result.Stdout(), "\r\n"), "\n", " ", -1)
	if actual != "search mydomain nameserver 127.0.0.1 options ndots:9" {
		c.Fatalf("expected 'search mydomain nameserver 127.0.0.1 options ndots:9', but says: %q", actual)
	}

	out := cli.DockerCmd(c, "run", "--dns=1.1.1.1", "--dns-search=.", "--dns-opt=ndots:3", "busybox", "cat", "/etc/resolv.conf").Combined()

	actual = strings.Replace(strings.Trim(strings.Trim(out, "\r\n"), " "), "\n", " ", -1)
	if actual != "nameserver 1.1.1.1 options ndots:3" {
		c.Fatalf("expected 'nameserver 1.1.1.1 options ndots:3', but says: %q", actual)
	}
}

func (s *DockerSuite) TestRunDNSRepeatOptions(c *testing.T) {
	testRequires(c, DaemonIsLinux)
	out := cli.DockerCmd(c, "run", "--dns=1.1.1.1", "--dns=2.2.2.2", "--dns-search=mydomain", "--dns-search=mydomain2", "--dns-opt=ndots:9", "--dns-opt=timeout:3", "busybox", "cat", "/etc/resolv.conf").Stdout()

	actual := strings.Replace(strings.Trim(out, "\r\n"), "\n", " ", -1)
	if actual != "search mydomain mydomain2 nameserver 1.1.1.1 nameserver 2.2.2.2 options ndots:9 timeout:3" {
		c.Fatalf("expected 'search mydomain mydomain2 nameserver 1.1.1.1 nameserver 2.2.2.2 options ndots:9 timeout:3', but says: %q", actual)
	}
}

func (s *DockerSuite) TestRunDNSOptionsBasedOnHostResolvConf(c *testing.T) {
	// Not applicable on Windows as testing Unix specific functionality
	testRequires(c, testEnv.IsLocalDaemon, DaemonIsLinux)

	origResolvConf, err := ioutil.ReadFile("/etc/resolv.conf")
	if os.IsNotExist(err) {
		c.Fatalf("/etc/resolv.conf does not exist")
	}

	hostNameservers := resolvconf.GetNameservers(origResolvConf, types.IP)
	hostSearch := resolvconf.GetSearchDomains(origResolvConf)

	var out string
	out, _ = dockerCmd(c, "run", "--dns=127.0.0.1", "busybox", "cat", "/etc/resolv.conf")

	if actualNameservers := resolvconf.GetNameservers([]byte(out), types.IP); actualNameservers[0] != "127.0.0.1" {
		c.Fatalf("expected '127.0.0.1', but says: %q", actualNameservers[0])
	}

	actualSearch := resolvconf.GetSearchDomains([]byte(out))
	if len(actualSearch) != len(hostSearch) {
		c.Fatalf("expected %q search domain(s), but it has: %q", len(hostSearch), len(actualSearch))
	}
	for i := range actualSearch {
		if actualSearch[i] != hostSearch[i] {
			c.Fatalf("expected %q domain, but says: %q", actualSearch[i], hostSearch[i])
		}
	}

	out, _ = dockerCmd(c, "run", "--dns-search=mydomain", "busybox", "cat", "/etc/resolv.conf")

	actualNameservers := resolvconf.GetNameservers([]byte(out), types.IP)
	if len(actualNameservers) != len(hostNameservers) {
		c.Fatalf("expected %q nameserver(s), but it has: %q", len(hostNameservers), len(actualNameservers))
	}
	for i := range actualNameservers {
		if actualNameservers[i] != hostNameservers[i] {
			c.Fatalf("expected %q nameserver, but says: %q", actualNameservers[i], hostNameservers[i])
		}
	}

	if actualSearch = resolvconf.GetSearchDomains([]byte(out)); actualSearch[0] != "mydomain" {
		c.Fatalf("expected 'mydomain', but says: %q", actualSearch[0])
	}

	// test with file
	tmpResolvConf := []byte("search example.com\nnameserver 12.34.56.78\nnameserver 127.0.0.1")
	if err := ioutil.WriteFile("/etc/resolv.conf", tmpResolvConf, 0644); err != nil {
		c.Fatal(err)
	}
	// put the old resolvconf back
	defer func() {
		if err := ioutil.WriteFile("/etc/resolv.conf", origResolvConf, 0644); err != nil {
			c.Fatal(err)
		}
	}()

	resolvConf, err := ioutil.ReadFile("/etc/resolv.conf")
	if os.IsNotExist(err) {
		c.Fatalf("/etc/resolv.conf does not exist")
	}

	hostSearch = resolvconf.GetSearchDomains(resolvConf)

	out, _ = dockerCmd(c, "run", "busybox", "cat", "/etc/resolv.conf")
	if actualNameservers = resolvconf.GetNameservers([]byte(out), types.IP); actualNameservers[0] != "12.34.56.78" || len(actualNameservers) != 1 {
		c.Fatalf("expected '12.34.56.78', but has: %v", actualNameservers)
	}

	actualSearch = resolvconf.GetSearchDomains([]byte(out))
	if len(actualSearch) != len(hostSearch) {
		c.Fatalf("expected %q search domain(s), but it has: %q", len(hostSearch), len(actualSearch))
	}
	for i := range actualSearch {
		if actualSearch[i] != hostSearch[i] {
			c.Fatalf("expected %q domain, but says: %q", actualSearch[i], hostSearch[i])
		}
	}
}

// Test to see if a non-root user can resolve a DNS name. Also
// check if the container resolv.conf file has at least 0644 perm.
func (s *DockerSuite) TestRunNonRootUserResolvName(c *testing.T) {
	// Not applicable on Windows as Windows does not support --user
	testRequires(c, testEnv.IsLocalDaemon, Network, DaemonIsLinux, NotArm)

	dockerCmd(c, "run", "--name=testperm", "--user=nobody", "busybox", "nslookup", "example.com")

	cID := getIDByName(c, "testperm")

	fmode := (os.FileMode)(0644)
	finfo, err := os.Stat(containerStorageFile(cID, "resolv.conf"))
	if err != nil {
		c.Fatal(err)
	}

	if (finfo.Mode() & fmode) != fmode {
		c.Fatalf("Expected container resolv.conf mode to be at least %s, instead got %s", fmode.String(), finfo.Mode().String())
	}
}

// Test if container resolv.conf gets updated the next time it restarts
// if host /etc/resolv.conf has changed. This only applies if the container
// uses the host's /etc/resolv.conf and does not have any dns options provided.
func (s *DockerSuite) TestRunResolvconfUpdate(c *testing.T) {
	// Not applicable on Windows as testing unix specific functionality
	testRequires(c, testEnv.IsLocalDaemon, DaemonIsLinux)
	c.Skip("Unstable test, to be re-activated once #19937 is resolved")

	tmpResolvConf := []byte("search pommesfrites.fr\nnameserver 12.34.56.78\n")
	tmpLocalhostResolvConf := []byte("nameserver 127.0.0.1")

	// take a copy of resolv.conf for restoring after test completes
	resolvConfSystem, err := ioutil.ReadFile("/etc/resolv.conf")
	if err != nil {
		c.Fatal(err)
	}

	// This test case is meant to test monitoring resolv.conf when it is
	// a regular file not a bind mounc. So we unmount resolv.conf and replace
	// it with a file containing the original settings.
	mounted, err := mountinfo.Mounted("/etc/resolv.conf")
	if err != nil {
		c.Fatal(err)
	}
	if mounted {
		icmd.RunCommand("umount", "/etc/resolv.conf").Assert(c, icmd.Success)
	}

	// cleanup
	defer func() {
		if err := ioutil.WriteFile("/etc/resolv.conf", resolvConfSystem, 0644); err != nil {
			c.Fatal(err)
		}
	}()

	// 1. test that a restarting container gets an updated resolv.conf
	dockerCmd(c, "run", "--name=first", "busybox", "true")
	containerID1 := getIDByName(c, "first")

	// replace resolv.conf with our temporary copy
	if err := ioutil.WriteFile("/etc/resolv.conf", tmpResolvConf, 0644); err != nil {
		c.Fatal(err)
	}

	// start the container again to pickup changes
	dockerCmd(c, "start", "first")

	// check for update in container
	containerResolv := readContainerFile(c, containerID1, "resolv.conf")
	if !bytes.Equal(containerResolv, tmpResolvConf) {
		c.Fatalf("Restarted container does not have updated resolv.conf; expected %q, got %q", tmpResolvConf, string(containerResolv))
	}

	/*	// make a change to resolv.conf (in this case replacing our tmp copy with orig copy)
		if err := ioutil.WriteFile("/etc/resolv.conf", resolvConfSystem, 0644); err != nil {
						c.Fatal(err)
								} */
	// 2. test that a restarting container does not receive resolv.conf updates
	//   if it modified the container copy of the starting point resolv.conf
	dockerCmd(c, "run", "--name=second", "busybox", "sh", "-c", "echo 'search mylittlepony.com' >>/etc/resolv.conf")
	containerID2 := getIDByName(c, "second")

	// make a change to resolv.conf (in this case replacing our tmp copy with orig copy)
	if err := ioutil.WriteFile("/etc/resolv.conf", resolvConfSystem, 0644); err != nil {
		c.Fatal(err)
	}

	// start the container again
	dockerCmd(c, "start", "second")

	// check for update in container
	containerResolv = readContainerFile(c, containerID2, "resolv.conf")
	if bytes.Equal(containerResolv, resolvConfSystem) {
		c.Fatalf("Container's resolv.conf should not have been updated with host resolv.conf: %q", string(containerResolv))
	}

	// 3. test that a running container's resolv.conf is not modified while running
	out, _ := dockerCmd(c, "run", "-d", "busybox", "top")
	runningContainerID := strings.TrimSpace(out)

	// replace resolv.conf
	if err := ioutil.WriteFile("/etc/resolv.conf", tmpResolvConf, 0644); err != nil {
		c.Fatal(err)
	}

	// check for update in container
	containerResolv = readContainerFile(c, runningContainerID, "resolv.conf")
	if bytes.Equal(containerResolv, tmpResolvConf) {
		c.Fatalf("Running container should not have updated resolv.conf; expected %q, got %q", string(resolvConfSystem), string(containerResolv))
	}

	// 4. test that a running container's resolv.conf is updated upon restart
	//   (the above container is still running..)
	dockerCmd(c, "restart", runningContainerID)

	// check for update in container
	containerResolv = readContainerFile(c, runningContainerID, "resolv.conf")
	if !bytes.Equal(containerResolv, tmpResolvConf) {
		c.Fatalf("Restarted container should have updated resolv.conf; expected %q, got %q", string(tmpResolvConf), string(containerResolv))
	}

	// 5. test that additions of a localhost resolver are cleaned from
	//   host resolv.conf before updating container's resolv.conf copies

	// replace resolv.conf with a localhost-only nameserver copy
	if err = ioutil.WriteFile("/etc/resolv.conf", tmpLocalhostResolvConf, 0644); err != nil {
		c.Fatal(err)
	}

	// start the container again to pickup changes
	dockerCmd(c, "start", "first")

	// our first exited container ID should have been updated, but with default DNS
	// after the cleanup of resolv.conf found only a localhost nameserver:
	containerResolv = readContainerFile(c, containerID1, "resolv.conf")
	expected := "\nnameserver 8.8.8.8\nnameserver 8.8.4.4\n"
	if !bytes.Equal(containerResolv, []byte(expected)) {
		c.Fatalf("Container does not have cleaned/replaced DNS in resolv.conf; expected %q, got %q", expected, string(containerResolv))
	}

	// 6. Test that replacing (as opposed to modifying) resolv.conf triggers an update
	//   of containers' resolv.conf.

	// Restore the original resolv.conf
	if err := ioutil.WriteFile("/etc/resolv.conf", resolvConfSystem, 0644); err != nil {
		c.Fatal(err)
	}

	// Run the container so it picks up the old settings
	dockerCmd(c, "run", "--name=third", "busybox", "true")
	containerID3 := getIDByName(c, "third")

	// Create a modified resolv.conf.aside and override resolv.conf with it
	if err := ioutil.WriteFile("/etc/resolv.conf.aside", tmpResolvConf, 0644); err != nil {
		c.Fatal(err)
	}

	err = os.Rename("/etc/resolv.conf.aside", "/etc/resolv.conf")
	if err != nil {
		c.Fatal(err)
	}

	// start the container again to pickup changes
	dockerCmd(c, "start", "third")

	// check for update in container
	containerResolv = readContainerFile(c, containerID3, "resolv.conf")
	if !bytes.Equal(containerResolv, tmpResolvConf) {
		c.Fatalf("Stopped container does not have updated resolv.conf; expected\n%q\n got\n%q", tmpResolvConf, string(containerResolv))
	}

	// cleanup, restore original resolv.conf happens in defer func()
}

func (s *DockerSuite) TestRunAddHost(c *testing.T) {
	// Not applicable on Windows as it does not support --add-host
	testRequires(c, DaemonIsLinux)
	out, _ := dockerCmd(c, "run", "--add-host=extra:86.75.30.9", "busybox", "grep", "extra", "/etc/hosts")

	actual := strings.Trim(out, "\r\n")
	if actual != "86.75.30.9\textra" {
		c.Fatalf("expected '86.75.30.9\textra', but says: %q", actual)
	}
}

// Regression test for #6983
func (s *DockerSuite) TestRunAttachStdErrOnlyTTYMode(c *testing.T) {
	_, exitCode := dockerCmd(c, "run", "-t", "-a", "stderr", "busybox", "true")
	if exitCode != 0 {
		c.Fatalf("Container should have exited with error code 0")
	}
}

// Regression test for #6983
func (s *DockerSuite) TestRunAttachStdOutOnlyTTYMode(c *testing.T) {
	_, exitCode := dockerCmd(c, "run", "-t", "-a", "stdout", "busybox", "true")
	if exitCode != 0 {
		c.Fatalf("Container should have exited with error code 0")
	}
}

// Regression test for #6983
func (s *DockerSuite) TestRunAttachStdOutAndErrTTYMode(c *testing.T) {
	_, exitCode := dockerCmd(c, "run", "-t", "-a", "stdout", "-a", "stderr", "busybox", "true")
	if exitCode != 0 {
		c.Fatalf("Container should have exited with error code 0")
	}
}

// Test for #10388 - this will run the same test as TestRunAttachStdOutAndErrTTYMode
// but using --attach instead of -a to make sure we read the flag correctly
func (s *DockerSuite) TestRunAttachWithDetach(c *testing.T) {
	icmd.RunCommand(dockerBinary, "run", "-d", "--attach", "stdout", "busybox", "true").Assert(c, icmd.Expected{
		ExitCode: 1,
		Error:    "exit status 1",
		Err:      "Conflicting options: -a and -d",
	})
}

func (s *DockerSuite) TestRunState(c *testing.T) {
	// TODO Windows: This needs some rework as Windows busybox does not support top
	testRequires(c, DaemonIsLinux)
	out, _ := dockerCmd(c, "run", "-d", "busybox", "top")

	id := strings.TrimSpace(out)
	state := inspectField(c, id, "State.Running")
	if state != "true" {
		c.Fatal("Container state is 'not running'")
	}
	pid1 := inspectField(c, id, "State.Pid")
	if pid1 == "0" {
		c.Fatal("Container state Pid 0")
	}

	dockerCmd(c, "stop", id)
	state = inspectField(c, id, "State.Running")
	if state != "false" {
		c.Fatal("Container state is 'running'")
	}
	pid2 := inspectField(c, id, "State.Pid")
	if pid2 == pid1 {
		c.Fatalf("Container state Pid %s, but expected %s", pid2, pid1)
	}

	dockerCmd(c, "start", id)
	state = inspectField(c, id, "State.Running")
	if state != "true" {
		c.Fatal("Container state is 'not running'")
	}
	pid3 := inspectField(c, id, "State.Pid")
	if pid3 == pid1 {
		c.Fatalf("Container state Pid %s, but expected %s", pid2, pid1)
	}
}

// Test for #1737
func (s *DockerSuite) TestRunCopyVolumeUIDGID(c *testing.T) {
	// Not applicable on Windows as it does not support uid or gid in this way
	testRequires(c, DaemonIsLinux)
	name := "testrunvolumesuidgid"
	buildImageSuccessfully(c, name, build.WithDockerfile(`FROM busybox
		RUN echo 'dockerio:x:1001:1001::/bin:/bin/false' >> /etc/passwd
		RUN echo 'dockerio:x:1001:' >> /etc/group
		RUN mkdir -p /hello && touch /hello/test && chown dockerio.dockerio /hello`))

	// Test that the uid and gid is copied from the image to the volume
	out, _ := dockerCmd(c, "run", "--rm", "-v", "/hello", name, "sh", "-c", "ls -l / | grep hello | awk '{print $3\":\"$4}'")
	out = strings.TrimSpace(out)
	if out != "dockerio:dockerio" {
		c.Fatalf("Wrong /hello ownership: %s, expected dockerio:dockerio", out)
	}
}

// Test for #1582
func (s *DockerSuite) TestRunCopyVolumeContent(c *testing.T) {
	// TODO Windows, post RS1. Windows does not yet support volume functionality
	// that copies from the image to the volume.
	testRequires(c, DaemonIsLinux)
	name := "testruncopyvolumecontent"
	buildImageSuccessfully(c, name, build.WithDockerfile(`FROM busybox
		RUN mkdir -p /hello/local && echo hello > /hello/local/world`))

	// Test that the content is copied from the image to the volume
	out, _ := dockerCmd(c, "run", "--rm", "-v", "/hello", name, "find", "/hello")
	if !(strings.Contains(out, "/hello/local/world") && strings.Contains(out, "/hello/local")) {
		c.Fatal("Container failed to transfer content to volume")
	}
}

func (s *DockerSuite) TestRunCleanupCmdOnEntrypoint(c *testing.T) {
	name := "testrunmdcleanuponentrypoint"
	buildImageSuccessfully(c, name, build.WithDockerfile(`FROM busybox
		ENTRYPOINT ["echo"]
		CMD ["testingpoint"]`))

	out, exit := dockerCmd(c, "run", "--entrypoint", "whoami", name)
	if exit != 0 {
		c.Fatalf("expected exit code 0 received %d, out: %q", exit, out)
	}
	out = strings.TrimSpace(out)
	expected := "root"
	if testEnv.OSType == "windows" {
		if strings.Contains(testEnv.PlatformDefaults.BaseImage, "servercore") {
			expected = `user manager\containeradministrator`
		} else {
			expected = `ContainerAdministrator` // nanoserver
		}
	}
	if out != expected {
		c.Fatalf("Expected output %s, got %q. %s", expected, out, testEnv.PlatformDefaults.BaseImage)
	}
}

// TestRunWorkdirExistsAndIsFile checks that if 'docker run -w' with existing file can be detected
func (s *DockerSuite) TestRunWorkdirExistsAndIsFile(c *testing.T) {
	existingFile := "/bin/cat"
	expected := "not a directory"
	if testEnv.OSType == "windows" {
		existingFile = `\windows\system32\ntdll.dll`
		expected = `The directory name is invalid.`
	}

	out, exitCode, err := dockerCmdWithError("run", "-w", existingFile, "busybox")
	if !(err != nil && exitCode == 125 && strings.Contains(out, expected)) {
		c.Fatalf("Existing binary as a directory should error out with exitCode 125; we got: %s, exitCode: %d", out, exitCode)
	}
}

func (s *DockerSuite) TestRunExitOnStdinClose(c *testing.T) {
	name := "testrunexitonstdinclose"

	meow := "/bin/cat"
	delay := 60
	if testEnv.OSType == "windows" {
		meow = "cat"
	}
	runCmd := exec.Command(dockerBinary, "run", "--name", name, "-i", "busybox", meow)

	stdin, err := runCmd.StdinPipe()
	if err != nil {
		c.Fatal(err)
	}
	stdout, err := runCmd.StdoutPipe()
	if err != nil {
		c.Fatal(err)
	}

	if err := runCmd.Start(); err != nil {
		c.Fatal(err)
	}
	if _, err := stdin.Write([]byte("hello\n")); err != nil {
		c.Fatal(err)
	}

	r := bufio.NewReader(stdout)
	line, err := r.ReadString('\n')
	if err != nil {
		c.Fatal(err)
	}
	line = strings.TrimSpace(line)
	if line != "hello" {
		c.Fatalf("Output should be 'hello', got '%q'", line)
	}
	if err := stdin.Close(); err != nil {
		c.Fatal(err)
	}
	finish := make(chan error, 1)
	go func() {
		finish <- runCmd.Wait()
		close(finish)
	}()
	select {
	case err := <-finish:
		assert.NilError(c, err)
	case <-time.After(time.Duration(delay) * time.Second):
		c.Fatal("docker run failed to exit on stdin close")
	}
	state := inspectField(c, name, "State.Running")

	if state != "false" {
		c.Fatal("Container must be stopped after stdin closing")
	}
}

// Test run -i --restart xxx doesn't hang
func (s *DockerSuite) TestRunInteractiveWithRestartPolicy(c *testing.T) {
	name := "test-inter-restart"

	result := icmd.RunCmd(icmd.Cmd{
		Command: []string{dockerBinary, "run", "-i", "--name", name, "--restart=always", "busybox", "sh"},
		Stdin:   bytes.NewBufferString("exit 11"),
	})
	defer func() {
		cli.Docker(cli.Args("stop", name)).Assert(c, icmd.Success)
	}()

	result.Assert(c, icmd.Expected{ExitCode: 11})
}

// Test for #2267
func (s *DockerSuite) TestRunWriteSpecialFilesAndNotCommit(c *testing.T) {
	// Cannot run on Windows as this files are not present in Windows
	testRequires(c, DaemonIsLinux)

	testRunWriteSpecialFilesAndNotCommit(c, "writehosts", "/etc/hosts")
	testRunWriteSpecialFilesAndNotCommit(c, "writehostname", "/etc/hostname")
	testRunWriteSpecialFilesAndNotCommit(c, "writeresolv", "/etc/resolv.conf")
}

func testRunWriteSpecialFilesAndNotCommit(c *testing.T, name, path string) {
	command := fmt.Sprintf("echo test2267 >> %s && cat %s", path, path)
	out, _ := dockerCmd(c, "run", "--name", name, "busybox", "sh", "-c", command)
	if !strings.Contains(out, "test2267") {
		c.Fatalf("%s should contain 'test2267'", path)
	}

	out, _ = dockerCmd(c, "diff", name)
	if len(strings.Trim(out, "\r\n")) != 0 && !eqToBaseDiff(out, c) {
		c.Fatal("diff should be empty")
	}
}

func eqToBaseDiff(out string, c *testing.T) bool {
	name := "eqToBaseDiff" + testutil.GenerateRandomAlphaOnlyString(32)
	dockerCmd(c, "run", "--name", name, "busybox", "echo", "hello")
	cID := getIDByName(c, name)
	baseDiff, _ := dockerCmd(c, "diff", cID)
	baseArr := strings.Split(baseDiff, "\n")
	sort.Strings(baseArr)
	outArr := strings.Split(out, "\n")
	sort.Strings(outArr)
	return sliceEq(baseArr, outArr)
}

func sliceEq(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}

	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}

	return true
}

func (s *DockerSuite) TestRunWithBadDevice(c *testing.T) {
	// Cannot run on Windows as Windows does not support --device
	testRequires(c, DaemonIsLinux)
	name := "baddevice"
	out, _, err := dockerCmdWithError("run", "--name", name, "--device", "/etc", "busybox", "true")

	if err == nil {
		c.Fatal("Run should fail with bad device")
	}
	expected := `"/etc": not a device node`
	if !strings.Contains(out, expected) {
		c.Fatalf("Output should contain %q, actual out: %q", expected, out)
	}
}

func (s *DockerSuite) TestRunEntrypoint(c *testing.T) {
	name := "entrypoint"

	out, _ := dockerCmd(c, "run", "--name", name, "--entrypoint", "echo", "busybox", "-n", "foobar")
	expected := "foobar"

	if out != expected {
		c.Fatalf("Output should be %q, actual out: %q", expected, out)
	}
}

func (s *DockerSuite) TestRunBindMounts(c *testing.T) {
	testRequires(c, testEnv.IsLocalDaemon)
	if testEnv.OSType == "linux" {
		testRequires(c, DaemonIsLinux, NotUserNamespace)
	}

	if testEnv.OSType == "windows" {
		// Disabled prior to RS5 due to how volumes are mapped
		testRequires(c, DaemonIsWindowsAtLeastBuild(osversion.RS5))
	}

	prefix, _ := getPrefixAndSlashFromDaemonPlatform()

	tmpDir, err := ioutil.TempDir("", "docker-test-container")
	if err != nil {
		c.Fatal(err)
	}

	defer os.RemoveAll(tmpDir)
	writeFile(path.Join(tmpDir, "touch-me"), "", c)

	// Test reading from a read-only bind mount
	out, _ := dockerCmd(c, "run", "-v", fmt.Sprintf("%s:%s/tmpx:ro", tmpDir, prefix), "busybox", "ls", prefix+"/tmpx")
	if !strings.Contains(out, "touch-me") {
		c.Fatal("Container failed to read from bind mount")
	}

	// test writing to bind mount
	if testEnv.OSType == "windows" {
		dockerCmd(c, "run", "-v", fmt.Sprintf(`%s:c:\tmp:rw`, tmpDir), "busybox", "touch", "c:/tmp/holla")
	} else {
		dockerCmd(c, "run", "-v", fmt.Sprintf("%s:/tmp:rw", tmpDir), "busybox", "touch", "/tmp/holla")
	}

	readFile(path.Join(tmpDir, "holla"), c) // Will fail if the file doesn't exist

	// test mounting to an illegal destination directory
	_, _, err = dockerCmdWithError("run", "-v", fmt.Sprintf("%s:.", tmpDir), "busybox", "ls", ".")
	if err == nil {
		c.Fatal("Container bind mounted illegal directory")
	}

	// Windows does not (and likely never will) support mounting a single file
	if testEnv.OSType != "windows" {
		// test mount a file
		dockerCmd(c, "run", "-v", fmt.Sprintf("%s/holla:/tmp/holla:rw", tmpDir), "busybox", "sh", "-c", "echo -n 'yotta' > /tmp/holla")
		content := readFile(path.Join(tmpDir, "holla"), c) // Will fail if the file doesn't exist
		expected := "yotta"
		if content != expected {
			c.Fatalf("Output should be %q, actual out: %q", expected, content)
		}
	}
}

// Ensure that CIDFile gets deleted if it's empty
// Perform this test by making `docker run` fail
func (s *DockerSuite) TestRunCidFileCleanupIfEmpty(c *testing.T) {
	// Skip on Windows. Base image on Windows has a CMD set in the image.
	testRequires(c, DaemonIsLinux)

	tmpDir, err := ioutil.TempDir("", "TestRunCidFile")
	if err != nil {
		c.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)
	tmpCidFile := path.Join(tmpDir, "cid")

	image := "emptyfs"
	if testEnv.OSType == "windows" {
		// Windows can't support an emptyfs image. Just use the regular Windows image
		image = testEnv.PlatformDefaults.BaseImage
	}
	out, _, err := dockerCmdWithError("run", "--cidfile", tmpCidFile, image)
	if err == nil {
		c.Fatalf("Run without command must fail. out=%s", out)
	} else if !strings.Contains(out, "No command specified") {
		c.Fatalf("Run without command failed with wrong output. out=%s\nerr=%v", out, err)
	}

	if _, err := os.Stat(tmpCidFile); err == nil {
		c.Fatalf("empty CIDFile %q should've been deleted", tmpCidFile)
	}
}

// #2098 - Docker cidFiles only contain short version of the containerId
// sudo docker run --cidfile /tmp/docker_tesc.cid ubuntu echo "test"
// TestRunCidFile tests that run --cidfile returns the longid
func (s *DockerSuite) TestRunCidFileCheckIDLength(c *testing.T) {
	tmpDir, err := ioutil.TempDir("", "TestRunCidFile")
	if err != nil {
		c.Fatal(err)
	}
	tmpCidFile := path.Join(tmpDir, "cid")
	defer os.RemoveAll(tmpDir)

	out, _ := dockerCmd(c, "run", "-d", "--cidfile", tmpCidFile, "busybox", "true")

	id := strings.TrimSpace(out)
	buffer, err := ioutil.ReadFile(tmpCidFile)
	if err != nil {
		c.Fatal(err)
	}
	cid := string(buffer)
	if len(cid) != 64 {
		c.Fatalf("--cidfile should be a long id, not %q", id)
	}
	if cid != id {
		c.Fatalf("cid must be equal to %s, got %s", id, cid)
	}
}

func (s *DockerSuite) TestRunSetMacAddress(c *testing.T) {
	mac := "12:34:56:78:9a:bc"
	var out string
	if testEnv.OSType == "windows" {
		out, _ = dockerCmd(c, "run", "-i", "--rm", fmt.Sprintf("--mac-address=%s", mac), "busybox", "sh", "-c", "ipconfig /all | grep 'Physical Address' | awk '{print $12}'")
		mac = strings.Replace(strings.ToUpper(mac), ":", "-", -1) // To Windows-style MACs
	} else {
		out, _ = dockerCmd(c, "run", "-i", "--rm", fmt.Sprintf("--mac-address=%s", mac), "busybox", "/bin/sh", "-c", "ip link show eth0 | tail -1 | awk '{print $2}'")
	}

	actualMac := strings.TrimSpace(out)
	if actualMac != mac {
		c.Fatalf("Set MAC address with --mac-address failed. The container has an incorrect MAC address: %q, expected: %q", actualMac, mac)
	}
}

func (s *DockerSuite) TestRunInspectMacAddress(c *testing.T) {
	// TODO Windows. Network settings are not propagated back to inspect.
	testRequires(c, DaemonIsLinux)
	mac := "12:34:56:78:9a:bc"
	out, _ := dockerCmd(c, "run", "-d", "--mac-address="+mac, "busybox", "top")

	id := strings.TrimSpace(out)
	inspectedMac := inspectField(c, id, "NetworkSettings.Networks.bridge.MacAddress")
	if inspectedMac != mac {
		c.Fatalf("docker inspect outputs wrong MAC address: %q, should be: %q", inspectedMac, mac)
	}
}

// test docker run use an invalid mac address
func (s *DockerSuite) TestRunWithInvalidMacAddress(c *testing.T) {
	out, _, err := dockerCmdWithError("run", "--mac-address", "92:d0:c6:0a:29", "busybox")
	// use an invalid mac address should with an error out
	if err == nil || !strings.Contains(out, "is not a valid mac address") {
		c.Fatalf("run with an invalid --mac-address should with error out")
	}
}

func (s *DockerSuite) TestRunDeallocatePortOnMissingIptablesRule(c *testing.T) {
	// TODO Windows. Network settings are not propagated back to inspect.
	testRequires(c, testEnv.IsLocalDaemon, DaemonIsLinux)

	out := cli.DockerCmd(c, "run", "-d", "-p", "23:23", "busybox", "top").Combined()

	id := strings.TrimSpace(out)
	ip := inspectField(c, id, "NetworkSettings.Networks.bridge.IPAddress")
	icmd.RunCommand("iptables", "-D", "DOCKER", "-d", fmt.Sprintf("%s/32", ip),
		"!", "-i", "docker0", "-o", "docker0", "-p", "tcp", "-m", "tcp", "--dport", "23", "-j", "ACCEPT").Assert(c, icmd.Success)

	cli.DockerCmd(c, "rm", "-fv", id)

	cli.DockerCmd(c, "run", "-d", "-p", "23:23", "busybox", "top")
}

func (s *DockerSuite) TestRunPortInUse(c *testing.T) {
	// TODO Windows. The duplicate NAT message returned by Windows will be
	// changing as is currently completely undecipherable. Does need modifying
	// to run sh rather than top though as top isn't in Windows busybox.
	testRequires(c, testEnv.IsLocalDaemon, DaemonIsLinux)

	port := "1234"
	dockerCmd(c, "run", "-d", "-p", port+":80", "busybox", "top")

	out, _, err := dockerCmdWithError("run", "-d", "-p", port+":80", "busybox", "top")
	if err == nil {
		c.Fatalf("Binding on used port must fail")
	}
	if !strings.Contains(out, "port is already allocated") {
		c.Fatalf("Out must be about \"port is already allocated\", got %s", out)
	}
}

// https://github.com/docker/docker/issues/12148
func (s *DockerSuite) TestRunAllocatePortInReservedRange(c *testing.T) {
	// TODO Windows. -P is not yet supported
	testRequires(c, DaemonIsLinux)
	// allocate a dynamic port to get the most recent
	out, _ := dockerCmd(c, "run", "-d", "-P", "-p", "80", "busybox", "top")

	id := strings.TrimSpace(out)
	out, _ = dockerCmd(c, "port", id, "80")

	strPort := strings.Split(strings.TrimSpace(out), ":")[1]
	port, err := strconv.ParseInt(strPort, 10, 64)
	if err != nil {
		c.Fatalf("invalid port, got: %s, error: %s", strPort, err)
	}

	// allocate a static port and a dynamic port together, with static port
	// takes the next recent port in dynamic port range.
	dockerCmd(c, "run", "-d", "-P", "-p", "80", "-p", fmt.Sprintf("%d:8080", port+1), "busybox", "top")
}

// Regression test for #7792
func (s *DockerSuite) TestRunMountOrdering(c *testing.T) {
	// TODO Windows: Post RS1. Windows does not support nested mounts.
	testRequires(c, testEnv.IsLocalDaemon, DaemonIsLinux, NotUserNamespace)
	prefix, _ := getPrefixAndSlashFromDaemonPlatform()

	tmpDir, err := ioutil.TempDir("", "docker_nested_mount_test")
	if err != nil {
		c.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	tmpDir2, err := ioutil.TempDir("", "docker_nested_mount_test2")
	if err != nil {
		c.Fatal(err)
	}
	defer os.RemoveAll(tmpDir2)

	// Create a temporary tmpfs mounc.
	fooDir := filepath.Join(tmpDir, "foo")
	if err := os.MkdirAll(filepath.Join(tmpDir, "foo"), 0755); err != nil {
		c.Fatalf("failed to mkdir at %s - %s", fooDir, err)
	}

	if err := ioutil.WriteFile(fmt.Sprintf("%s/touch-me", fooDir), []byte{}, 0644); err != nil {
		c.Fatal(err)
	}

	if err := ioutil.WriteFile(fmt.Sprintf("%s/touch-me", tmpDir), []byte{}, 0644); err != nil {
		c.Fatal(err)
	}

	if err := ioutil.WriteFile(fmt.Sprintf("%s/touch-me", tmpDir2), []byte{}, 0644); err != nil {
		c.Fatal(err)
	}

	dockerCmd(c, "run",
		"-v", fmt.Sprintf("%s:"+prefix+"/tmp", tmpDir),
		"-v", fmt.Sprintf("%s:"+prefix+"/tmp/foo", fooDir),
		"-v", fmt.Sprintf("%s:"+prefix+"/tmp/tmp2", tmpDir2),
		"-v", fmt.Sprintf("%s:"+prefix+"/tmp/tmp2/foo", fooDir),
		"busybox:latest", "sh", "-c",
		"ls "+prefix+"/tmp/touch-me && ls "+prefix+"/tmp/foo/touch-me && ls "+prefix+"/tmp/tmp2/touch-me && ls "+prefix+"/tmp/tmp2/foo/touch-me")
}

// Regression test for https://github.com/docker/docker/issues/8259
func (s *DockerSuite) TestRunReuseBindVolumeThatIsSymlink(c *testing.T) {
	// Not applicable on Windows as Windows does not support volumes
	testRequires(c, testEnv.IsLocalDaemon, DaemonIsLinux, NotUserNamespace)
	prefix, _ := getPrefixAndSlashFromDaemonPlatform()

	tmpDir, err := ioutil.TempDir(os.TempDir(), "testlink")
	if err != nil {
		c.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	linkPath := os.TempDir() + "/testlink2"
	if err := os.Symlink(tmpDir, linkPath); err != nil {
		c.Fatal(err)
	}
	defer os.RemoveAll(linkPath)

	// Create first container
	dockerCmd(c, "run", "-v", fmt.Sprintf("%s:"+prefix+"/tmp/test", linkPath), "busybox", "ls", prefix+"/tmp/test")

	// Create second container with same symlinked path
	// This will fail if the referenced issue is hit with a "Volume exists" error
	dockerCmd(c, "run", "-v", fmt.Sprintf("%s:"+prefix+"/tmp/test", linkPath), "busybox", "ls", prefix+"/tmp/test")
}

// GH#10604: Test an "/etc" volume doesn't overlay special bind mounts in container
func (s *DockerSuite) TestRunCreateVolumeEtc(c *testing.T) {
	// While Windows supports volumes, it does not support --add-host hence
	// this test is not applicable on Windows.
	testRequires(c, DaemonIsLinux)
	out, _ := dockerCmd(c, "run", "--dns=127.0.0.1", "-v", "/etc", "busybox", "cat", "/etc/resolv.conf")
	if !strings.Contains(out, "nameserver 127.0.0.1") {
		c.Fatal("/etc volume mount hides /etc/resolv.conf")
	}

	out, _ = dockerCmd(c, "run", "-h=test123", "-v", "/etc", "busybox", "cat", "/etc/hostname")
	if !strings.Contains(out, "test123") {
		c.Fatal("/etc volume mount hides /etc/hostname")
	}

	out, _ = dockerCmd(c, "run", "--add-host=test:192.168.0.1", "-v", "/etc", "busybox", "cat", "/etc/hosts")
	out = strings.Replace(out, "\n", " ", -1)
	if !strings.Contains(out, "192.168.0.1\ttest") || !strings.Contains(out, "127.0.0.1\tlocalhost") {
		c.Fatal("/etc volume mount hides /etc/hosts")
	}
}

func (s *DockerSuite) TestVolumesNoCopyData(c *testing.T) {
	// TODO Windows (Post RS1). Windows does not support volumes which
	// are pre-populated such as is built in the dockerfile used in this test.
	testRequires(c, DaemonIsLinux)
	prefix, slash := getPrefixAndSlashFromDaemonPlatform()
	buildImageSuccessfully(c, "dataimage", build.WithDockerfile(`FROM busybox
		RUN ["mkdir", "-p", "/foo"]
		RUN ["touch", "/foo/bar"]`))
	dockerCmd(c, "run", "--name", "test", "-v", prefix+slash+"foo", "busybox")

	if out, _, err := dockerCmdWithError("run", "--volumes-from", "test", "dataimage", "ls", "-lh", "/foo/bar"); err == nil || !strings.Contains(out, "No such file or directory") {
		c.Fatalf("Data was copied on volumes-from but shouldn't be:\n%q", out)
	}

	tmpDir := RandomTmpDirPath("docker_test_bind_mount_copy_data", testEnv.OSType)
	if out, _, err := dockerCmdWithError("run", "-v", tmpDir+":/foo", "dataimage", "ls", "-lh", "/foo/bar"); err == nil || !strings.Contains(out, "No such file or directory") {
		c.Fatalf("Data was copied on bind mount but shouldn't be:\n%q", out)
	}
}

func (s *DockerSuite) TestRunNoOutputFromPullInStdout(c *testing.T) {
	// just run with unknown image
	cmd := exec.Command(dockerBinary, "run", "asdfsg")
	stdout := bytes.NewBuffer(nil)
	cmd.Stdout = stdout
	if err := cmd.Run(); err == nil {
		c.Fatal("Run with unknown image should fail")
	}
	if stdout.Len() != 0 {
		c.Fatalf("Stdout contains output from pull: %s", stdout)
	}
}

func (s *DockerSuite) TestRunVolumesCleanPaths(c *testing.T) {
	testRequires(c, testEnv.IsLocalDaemon)
	prefix, slash := getPrefixAndSlashFromDaemonPlatform()
	buildImageSuccessfully(c, "run_volumes_clean_paths", build.WithDockerfile(`FROM busybox
		VOLUME `+prefix+`/foo/`))
	dockerCmd(c, "run", "-v", prefix+"/foo", "-v", prefix+"/bar/", "--name", "dark_helmet", "run_volumes_clean_paths")

	out, err := inspectMountSourceField("dark_helmet", prefix+slash+"foo"+slash)
	if err != errMountNotFound {
		c.Fatalf("Found unexpected volume entry for '%s/foo/' in volumes\n%q", prefix, out)
	}

	out, err = inspectMountSourceField("dark_helmet", prefix+slash+`foo`)
	assert.NilError(c, err)
	if !strings.Contains(strings.ToLower(out), strings.ToLower(testEnv.PlatformDefaults.VolumesConfigPath)) {
		c.Fatalf("Volume was not defined for %s/foo\n%q", prefix, out)
	}

	out, err = inspectMountSourceField("dark_helmet", prefix+slash+"bar"+slash)
	if err != errMountNotFound {
		c.Fatalf("Found unexpected volume entry for '%s/bar/' in volumes\n%q", prefix, out)
	}

	out, err = inspectMountSourceField("dark_helmet", prefix+slash+"bar")
	assert.NilError(c, err)
	if !strings.Contains(strings.ToLower(out), strings.ToLower(testEnv.PlatformDefaults.VolumesConfigPath)) {
		c.Fatalf("Volume was not defined for %s/bar\n%q", prefix, out)
	}
}

// Regression test for #3631
func (s *DockerSuite) TestRunSlowStdoutConsumer(c *testing.T) {
	// TODO Windows: This should be able to run on Windows if can find an
	// alternate to /dev/zero and /dev/stdout.
	testRequires(c, DaemonIsLinux)

	args := []string{"run", "--rm", "busybox", "/bin/sh", "-c", "dd if=/dev/zero of=/dev/stdout bs=1024 count=2000 | cat -v"}
	cont := exec.Command(dockerBinary, args...)

	stdout, err := cont.StdoutPipe()
	if err != nil {
		c.Fatal(err)
	}

	if err := cont.Start(); err != nil {
		c.Fatal(err)
	}
	defer func() { go cont.Wait() }()
	n, err := ConsumeWithSpeed(stdout, 10000, 5*time.Millisecond, nil)
	if err != nil {
		c.Fatal(err)
	}

	expected := 2 * 1024 * 2000
	if n != expected {
		c.Fatalf("Expected %d, got %d", expected, n)
	}
}

func (s *DockerSuite) TestRunAllowPortRangeThroughExpose(c *testing.T) {
	// TODO Windows: -P is not currently supported. Also network
	// settings are not propagated back.
	testRequires(c, DaemonIsLinux)
	out, _ := dockerCmd(c, "run", "-d", "--expose", "3000-3003", "-P", "busybox", "top")

	id := strings.TrimSpace(out)
	portstr := inspectFieldJSON(c, id, "NetworkSettings.Ports")
	var ports nat.PortMap
	if err := json.Unmarshal([]byte(portstr), &ports); err != nil {
		c.Fatal(err)
	}
	for port, binding := range ports {
		portnum, _ := strconv.Atoi(strings.Split(string(port), "/")[0])
		if portnum < 3000 || portnum > 3003 {
			c.Fatalf("Port %d is out of range ", portnum)
		}
		if binding == nil || len(binding) != 1 || len(binding[0].HostPort) == 0 {
			c.Fatalf("Port is not mapped for the port %s", port)
		}
	}
}

func (s *DockerSuite) TestRunExposePort(c *testing.T) {
	out, _, err := dockerCmdWithError("run", "--expose", "80000", "busybox")
	assert.Assert(c, err != nil, "--expose with an invalid port should error out")
	assert.Assert(c, strings.Contains(out, "invalid range format for --expose"))
}

func (s *DockerSuite) TestRunModeIpcHost(c *testing.T) {
	// Not applicable on Windows as uses Unix-specific capabilities
	testRequires(c, testEnv.IsLocalDaemon, DaemonIsLinux, NotUserNamespace)

	hostIpc, err := os.Readlink("/proc/1/ns/ipc")
	if err != nil {
		c.Fatal(err)
	}

	out, _ := dockerCmd(c, "run", "--ipc=host", "busybox", "readlink", "/proc/self/ns/ipc")
	out = strings.Trim(out, "\n")
	if hostIpc != out {
		c.Fatalf("IPC different with --ipc=host %s != %s\n", hostIpc, out)
	}

	out, _ = dockerCmd(c, "run", "busybox", "readlink", "/proc/self/ns/ipc")
	out = strings.Trim(out, "\n")
	if hostIpc == out {
		c.Fatalf("IPC should be different without --ipc=host %s == %s\n", hostIpc, out)
	}
}

func (s *DockerSuite) TestRunModeIpcContainerNotExists(c *testing.T) {
	// Not applicable on Windows as uses Unix-specific capabilities
	testRequires(c, DaemonIsLinux)
	out, _, err := dockerCmdWithError("run", "-d", "--ipc", "container:abcd1234", "busybox", "top")
	if !strings.Contains(out, "abcd1234") || err == nil {
		c.Fatalf("run IPC from a non exists container should with correct error out")
	}
}

func (s *DockerSuite) TestRunModeIpcContainerNotRunning(c *testing.T) {
	// Not applicable on Windows as uses Unix-specific capabilities
	testRequires(c, testEnv.IsLocalDaemon, DaemonIsLinux)

	out, _ := dockerCmd(c, "create", "busybox")

	id := strings.TrimSpace(out)
	out, _, err := dockerCmdWithError("run", fmt.Sprintf("--ipc=container:%s", id), "busybox")
	if err == nil {
		c.Fatalf("Run container with ipc mode container should fail with non running container: %s\n%s", out, err)
	}
}

func (s *DockerSuite) TestRunModePIDContainer(c *testing.T) {
	// Not applicable on Windows as uses Unix-specific capabilities
	testRequires(c, testEnv.IsLocalDaemon, DaemonIsLinux)

	out, _ := dockerCmd(c, "run", "-d", "busybox", "sh", "-c", "top")

	id := strings.TrimSpace(out)
	state := inspectField(c, id, "State.Running")
	if state != "true" {
		c.Fatal("Container state is 'not running'")
	}
	pid1 := inspectField(c, id, "State.Pid")

	parentContainerPid, err := os.Readlink(fmt.Sprintf("/proc/%s/ns/pid", pid1))
	if err != nil {
		c.Fatal(err)
	}

	out, _ = dockerCmd(c, "run", fmt.Sprintf("--pid=container:%s", id), "busybox", "readlink", "/proc/self/ns/pid")
	out = strings.Trim(out, "\n")
	if parentContainerPid != out {
		c.Fatalf("PID different with --pid=container:%s %s != %s\n", id, parentContainerPid, out)
	}
}

func (s *DockerSuite) TestRunModePIDContainerNotExists(c *testing.T) {
	// Not applicable on Windows as uses Unix-specific capabilities
	testRequires(c, DaemonIsLinux)
	out, _, err := dockerCmdWithError("run", "-d", "--pid", "container:abcd1234", "busybox", "top")
	if !strings.Contains(out, "abcd1234") || err == nil {
		c.Fatalf("run PID from a non exists container should with correct error out")
	}
}

func (s *DockerSuite) TestRunModePIDContainerNotRunning(c *testing.T) {
	// Not applicable on Windows as uses Unix-specific capabilities
	testRequires(c, testEnv.IsLocalDaemon, DaemonIsLinux)

	out, _ := dockerCmd(c, "create", "busybox")

	id := strings.TrimSpace(out)
	out, _, err := dockerCmdWithError("run", fmt.Sprintf("--pid=container:%s", id), "busybox")
	if err == nil {
		c.Fatalf("Run container with pid mode container should fail with non running container: %s\n%s", out, err)
	}
}

func (s *DockerSuite) TestRunMountShmMqueueFromHost(c *testing.T) {
	// Not applicable on Windows as uses Unix-specific capabilities
	testRequires(c, testEnv.IsLocalDaemon, DaemonIsLinux, NotUserNamespace)

	dockerCmd(c, "run", "-d", "--name", "shmfromhost", "-v", "/dev/shm:/dev/shm", "-v", "/dev/mqueue:/dev/mqueue", "busybox", "sh", "-c", "echo -n test > /dev/shm/test && touch /dev/mqueue/toto && top")
	defer os.Remove("/dev/mqueue/toto")
	defer os.Remove("/dev/shm/test")
	volPath, err := inspectMountSourceField("shmfromhost", "/dev/shm")
	assert.NilError(c, err)
	if volPath != "/dev/shm" {
		c.Fatalf("volumePath should have been /dev/shm, was %s", volPath)
	}

	out, _ := dockerCmd(c, "run", "--name", "ipchost", "--ipc", "host", "busybox", "cat", "/dev/shm/test")
	if out != "test" {
		c.Fatalf("Output of /dev/shm/test expected test but found: %s", out)
	}

	// Check that the mq was created
	if _, err := os.Stat("/dev/mqueue/toto"); err != nil {
		c.Fatalf("Failed to confirm '/dev/mqueue/toto' presence on host: %s", err.Error())
	}
}

func (s *DockerSuite) TestContainerNetworkMode(c *testing.T) {
	// Not applicable on Windows as uses Unix-specific capabilities
	testRequires(c, testEnv.IsLocalDaemon, DaemonIsLinux)

	out, _ := dockerCmd(c, "run", "-d", "busybox", "top")
	id := strings.TrimSpace(out)
	assert.NilError(c, waitRun(id))
	pid1 := inspectField(c, id, "State.Pid")

	parentContainerNet, err := os.Readlink(fmt.Sprintf("/proc/%s/ns/net", pid1))
	if err != nil {
		c.Fatal(err)
	}

	out, _ = dockerCmd(c, "run", fmt.Sprintf("--net=container:%s", id), "busybox", "readlink", "/proc/self/ns/net")
	out = strings.Trim(out, "\n")
	if parentContainerNet != out {
		c.Fatalf("NET different with --net=container:%s %s != %s\n", id, parentContainerNet, out)
	}
}

func (s *DockerSuite) TestRunModePIDHost(c *testing.T) {
	// Not applicable on Windows as uses Unix-specific capabilities
	testRequires(c, testEnv.IsLocalDaemon, DaemonIsLinux, NotUserNamespace)

	hostPid, err := os.Readlink("/proc/1/ns/pid")
	if err != nil {
		c.Fatal(err)
	}

	out, _ := dockerCmd(c, "run", "--pid=host", "busybox", "readlink", "/proc/self/ns/pid")
	out = strings.Trim(out, "\n")
	if hostPid != out {
		c.Fatalf("PID different with --pid=host %s != %s\n", hostPid, out)
	}

	out, _ = dockerCmd(c, "run", "busybox", "readlink", "/proc/self/ns/pid")
	out = strings.Trim(out, "\n")
	if hostPid == out {
		c.Fatalf("PID should be different without --pid=host %s == %s\n", hostPid, out)
	}
}

func (s *DockerSuite) TestRunModeUTSHost(c *testing.T) {
	// Not applicable on Windows as uses Unix-specific capabilities
	testRequires(c, testEnv.IsLocalDaemon, DaemonIsLinux)

	hostUTS, err := os.Readlink("/proc/1/ns/uts")
	if err != nil {
		c.Fatal(err)
	}

	out, _ := dockerCmd(c, "run", "--uts=host", "busybox", "readlink", "/proc/self/ns/uts")
	out = strings.Trim(out, "\n")
	if hostUTS != out {
		c.Fatalf("UTS different with --uts=host %s != %s\n", hostUTS, out)
	}

	out, _ = dockerCmd(c, "run", "busybox", "readlink", "/proc/self/ns/uts")
	out = strings.Trim(out, "\n")
	if hostUTS == out {
		c.Fatalf("UTS should be different without --uts=host %s == %s\n", hostUTS, out)
	}

	out, _ = dockerCmdWithFail(c, "run", "-h=name", "--uts=host", "busybox", "ps")
	assert.Assert(c, strings.Contains(out, runconfig.ErrConflictUTSHostname.Error()))
}

func (s *DockerSuite) TestRunTLSVerify(c *testing.T) {
	// Remote daemons use TLS and this test is not applicable when TLS is required.
	testRequires(c, testEnv.IsLocalDaemon)
	if out, code, err := dockerCmdWithError("ps"); err != nil || code != 0 {
		c.Fatalf("Should have worked: %v:\n%v", err, out)
	}

	// Regardless of whether we specify true or false we need to
	// test to make sure tls is turned on if --tlsverify is specified at all
	result := dockerCmdWithResult("--tlsverify=false", "ps")
	result.Assert(c, icmd.Expected{ExitCode: 1, Err: "error during connect"})

	result = dockerCmdWithResult("--tlsverify=true", "ps")
	result.Assert(c, icmd.Expected{ExitCode: 1, Err: "cert"})
}

func (s *DockerSuite) TestRunPortFromDockerRangeInUse(c *testing.T) {
	// TODO Windows. Once moved to libnetwork/CNM, this may be able to be
	// re-instated.
	testRequires(c, DaemonIsLinux)
	// first find allocator current position
	out, _ := dockerCmd(c, "run", "-d", "-p", ":80", "busybox", "top")

	id := strings.TrimSpace(out)
	out, _ = dockerCmd(c, "port", id)

	out = strings.TrimSpace(out)
	if out == "" {
		c.Fatal("docker port command output is empty")
	}
	out = strings.Split(out, ":")[1]
	lastPort, err := strconv.Atoi(out)
	if err != nil {
		c.Fatal(err)
	}
	port := lastPort + 1
	l, err := net.Listen("tcp", ":"+strconv.Itoa(port))
	if err != nil {
		c.Fatal(err)
	}
	defer l.Close()

	out, _ = dockerCmd(c, "run", "-d", "-p", ":80", "busybox", "top")

	id = strings.TrimSpace(out)
	dockerCmd(c, "port", id)
}

func (s *DockerSuite) TestRunTTYWithPipe(c *testing.T) {
	errChan := make(chan error, 1)
	go func() {
		defer close(errChan)

		cmd := exec.Command(dockerBinary, "run", "-ti", "busybox", "true")
		if _, err := cmd.StdinPipe(); err != nil {
			errChan <- err
			return
		}

		expected := "the input device is not a TTY"
		if runtime.GOOS == "windows" {
			expected += ".  If you are using mintty, try prefixing the command with 'winpty'"
		}
		if out, _, err := runCommandWithOutput(cmd); err == nil {
			errChan <- fmt.Errorf("run should have failed")
			return
		} else if !strings.Contains(out, expected) {
			errChan <- fmt.Errorf("run failed with error %q: expected %q", out, expected)
			return
		}
	}()

	select {
	case err := <-errChan:
		assert.NilError(c, err)
	case <-time.After(30 * time.Second):
		c.Fatal("container is running but should have failed")
	}
}

func (s *DockerSuite) TestRunNonLocalMacAddress(c *testing.T) {
	addr := "00:16:3E:08:00:50"
	args := []string{"run", "--mac-address", addr}
	expected := addr

	if testEnv.OSType != "windows" {
		args = append(args, "busybox", "ifconfig")
	} else {
		args = append(args, testEnv.PlatformDefaults.BaseImage, "ipconfig", "/all")
		expected = strings.Replace(strings.ToUpper(addr), ":", "-", -1)
	}

	if out, _ := dockerCmd(c, args...); !strings.Contains(out, expected) {
		c.Fatalf("Output should have contained %q: %s", expected, out)
	}
}

func (s *DockerSuite) TestRunNetHost(c *testing.T) {
	// Not applicable on Windows as uses Unix-specific capabilities
	testRequires(c, testEnv.IsLocalDaemon, DaemonIsLinux, NotUserNamespace)

	hostNet, err := os.Readlink("/proc/1/ns/net")
	if err != nil {
		c.Fatal(err)
	}

	out, _ := dockerCmd(c, "run", "--net=host", "busybox", "readlink", "/proc/self/ns/net")
	out = strings.Trim(out, "\n")
	if hostNet != out {
		c.Fatalf("Net namespace different with --net=host %s != %s\n", hostNet, out)
	}

	out, _ = dockerCmd(c, "run", "busybox", "readlink", "/proc/self/ns/net")
	out = strings.Trim(out, "\n")
	if hostNet == out {
		c.Fatalf("Net namespace should be different without --net=host %s == %s\n", hostNet, out)
	}
}

func (s *DockerSuite) TestRunNetHostTwiceSameName(c *testing.T) {
	// TODO Windows. As Windows networking evolves and converges towards
	// CNM, this test may be possible to enable on Windows.
	testRequires(c, testEnv.IsLocalDaemon, DaemonIsLinux, NotUserNamespace)

	dockerCmd(c, "run", "--rm", "--name=thost", "--net=host", "busybox", "true")
	dockerCmd(c, "run", "--rm", "--name=thost", "--net=host", "busybox", "true")
}

func (s *DockerSuite) TestRunNetContainerWhichHost(c *testing.T) {
	// Not applicable on Windows as uses Unix-specific capabilities
	testRequires(c, testEnv.IsLocalDaemon, DaemonIsLinux, NotUserNamespace)

	hostNet, err := os.Readlink("/proc/1/ns/net")
	if err != nil {
		c.Fatal(err)
	}

	dockerCmd(c, "run", "-d", "--net=host", "--name=test", "busybox", "top")

	out, _ := dockerCmd(c, "run", "--net=container:test", "busybox", "readlink", "/proc/self/ns/net")
	out = strings.Trim(out, "\n")
	if hostNet != out {
		c.Fatalf("Container should have host network namespace")
	}
}

func (s *DockerSuite) TestRunAllowPortRangeThroughPublish(c *testing.T) {
	// TODO Windows. This may be possible to enable in the future. However,
	// Windows does not currently support --expose, or populate the network
	// settings seen through inspect.
	testRequires(c, DaemonIsLinux)
	out, _ := dockerCmd(c, "run", "-d", "--expose", "3000-3003", "-p", "3000-3003", "busybox", "top")

	id := strings.TrimSpace(out)
	portstr := inspectFieldJSON(c, id, "NetworkSettings.Ports")

	var ports nat.PortMap
	err := json.Unmarshal([]byte(portstr), &ports)
	assert.NilError(c, err, "failed to unmarshal: %v", portstr)
	for port, binding := range ports {
		portnum, _ := strconv.Atoi(strings.Split(string(port), "/")[0])
		if portnum < 3000 || portnum > 3003 {
			c.Fatalf("Port %d is out of range ", portnum)
		}
		if binding == nil || len(binding) != 1 || len(binding[0].HostPort) == 0 {
			c.Fatal("Port is not mapped for the port "+port, out)
		}
	}
}

func (s *DockerSuite) TestRunSetDefaultRestartPolicy(c *testing.T) {
	runSleepingContainer(c, "--name=testrunsetdefaultrestartpolicy")
	out := inspectField(c, "testrunsetdefaultrestartpolicy", "HostConfig.RestartPolicy.Name")
	if out != "no" {
		c.Fatalf("Set default restart policy failed")
	}
}

func (s *DockerSuite) TestRunRestartMaxRetries(c *testing.T) {
	out, _ := dockerCmd(c, "run", "-d", "--restart=on-failure:3", "busybox", "false")
	timeout := 10 * time.Second
	if testEnv.OSType == "windows" {
		timeout = 120 * time.Second
	}

	id := strings.TrimSpace(out)
	if err := waitInspect(id, "{{ .State.Restarting }} {{ .State.Running }}", "false false", timeout); err != nil {
		c.Fatal(err)
	}

	count := inspectField(c, id, "RestartCount")
	if count != "3" {
		c.Fatalf("Container was restarted %s times, expected %d", count, 3)
	}

	MaximumRetryCount := inspectField(c, id, "HostConfig.RestartPolicy.MaximumRetryCount")
	if MaximumRetryCount != "3" {
		c.Fatalf("Container Maximum Retry Count is %s, expected %s", MaximumRetryCount, "3")
	}
}

func (s *DockerSuite) TestRunContainerWithWritableRootfs(c *testing.T) {
	dockerCmd(c, "run", "--rm", "busybox", "touch", "/file")
}

func (s *DockerSuite) TestRunContainerWithReadonlyRootfs(c *testing.T) {
	// Not applicable on Windows which does not support --read-only
	testRequires(c, DaemonIsLinux, UserNamespaceROMount)

	testPriv := true
	// don't test privileged mode subtest if user namespaces enabled
	if root := os.Getenv("DOCKER_REMAP_ROOT"); root != "" {
		testPriv = false
	}
	testReadOnlyFile(c, testPriv, "/file", "/etc/hosts", "/etc/resolv.conf", "/etc/hostname")
}

func (s *DockerSuite) TestPermissionsPtsReadonlyRootfs(c *testing.T) {
	// Not applicable on Windows due to use of Unix specific functionality, plus
	// the use of --read-only which is not supported.
	testRequires(c, DaemonIsLinux, UserNamespaceROMount)

	// Ensure we have not broken writing /dev/pts
	out, status := dockerCmd(c, "run", "--read-only", "--rm", "busybox", "mount")
	if status != 0 {
		c.Fatal("Could not obtain mounts when checking /dev/pts mntpnt.")
	}
	expected := "type devpts (rw,"
	if !strings.Contains(out, expected) {
		c.Fatalf("expected output to contain %s but contains %s", expected, out)
	}
}

func testReadOnlyFile(c *testing.T, testPriv bool, filenames ...string) {
	touch := "touch " + strings.Join(filenames, " ")
	out, _, err := dockerCmdWithError("run", "--read-only", "--rm", "busybox", "sh", "-c", touch)
	assert.ErrorContains(c, err, "")

	for _, f := range filenames {
		expected := "touch: " + f + ": Read-only file system"
		assert.Assert(c, strings.Contains(out, expected))
	}

	if !testPriv {
		return
	}

	out, _, err = dockerCmdWithError("run", "--read-only", "--privileged", "--rm", "busybox", "sh", "-c", touch)
	assert.ErrorContains(c, err, "")

	for _, f := range filenames {
		expected := "touch: " + f + ": Read-only file system"
		assert.Assert(c, strings.Contains(out, expected))
	}
}

func (s *DockerSuite) TestRunContainerWithReadonlyEtcHostsAndLinkedContainer(c *testing.T) {
	// Not applicable on Windows which does not support --link
	testRequires(c, DaemonIsLinux, UserNamespaceROMount)

	dockerCmd(c, "run", "-d", "--name", "test-etc-hosts-ro-linked", "busybox", "top")

	out, _ := dockerCmd(c, "run", "--read-only", "--link", "test-etc-hosts-ro-linked:testlinked", "busybox", "cat", "/etc/hosts")
	if !strings.Contains(out, "testlinked") {
		c.Fatal("Expected /etc/hosts to be updated even if --read-only enabled")
	}
}

func (s *DockerSuite) TestRunContainerWithReadonlyRootfsWithDNSFlag(c *testing.T) {
	// Not applicable on Windows which does not support either --read-only or --dns.
	testRequires(c, DaemonIsLinux, UserNamespaceROMount)

	out, _ := dockerCmd(c, "run", "--read-only", "--dns", "1.1.1.1", "busybox", "/bin/cat", "/etc/resolv.conf")
	if !strings.Contains(out, "1.1.1.1") {
		c.Fatal("Expected /etc/resolv.conf to be updated even if --read-only enabled and --dns flag used")
	}
}

func (s *DockerSuite) TestRunContainerWithReadonlyRootfsWithAddHostFlag(c *testing.T) {
	// Not applicable on Windows which does not support --read-only
	testRequires(c, DaemonIsLinux, UserNamespaceROMount)

	out, _ := dockerCmd(c, "run", "--read-only", "--add-host", "testreadonly:127.0.0.1", "busybox", "/bin/cat", "/etc/hosts")
	if !strings.Contains(out, "testreadonly") {
		c.Fatal("Expected /etc/hosts to be updated even if --read-only enabled and --add-host flag used")
	}
}

func (s *DockerSuite) TestRunVolumesFromRestartAfterRemoved(c *testing.T) {
	prefix, _ := getPrefixAndSlashFromDaemonPlatform()
	runSleepingContainer(c, "--name=voltest", "-v", prefix+"/foo")
	runSleepingContainer(c, "--name=restarter", "--volumes-from", "voltest")

	// Remove the main volume container and restart the consuming container
	dockerCmd(c, "rm", "-f", "voltest")

	// This should not fail since the volumes-from were already applied
	dockerCmd(c, "restart", "restarter")
}

// run container with --rm should remove container if exit code != 0
func (s *DockerSuite) TestRunContainerWithRmFlagExitCodeNotEqualToZero(c *testing.T) {
	existingContainers := ExistingContainerIDs(c)
	name := "flowers"
	cli.Docker(cli.Args("run", "--name", name, "--rm", "busybox", "ls", "/notexists")).Assert(c, icmd.Expected{
		ExitCode: 1,
	})

	out := cli.DockerCmd(c, "ps", "-q", "-a").Combined()
	out = RemoveOutputForExistingElements(out, existingContainers)
	if out != "" {
		c.Fatal("Expected not to have containers", out)
	}
}

func (s *DockerSuite) TestRunContainerWithRmFlagCannotStartContainer(c *testing.T) {
	existingContainers := ExistingContainerIDs(c)
	name := "sparkles"
	cli.Docker(cli.Args("run", "--name", name, "--rm", "busybox", "commandNotFound")).Assert(c, icmd.Expected{
		ExitCode: 127,
	})
	out := cli.DockerCmd(c, "ps", "-q", "-a").Combined()
	out = RemoveOutputForExistingElements(out, existingContainers)
	if out != "" {
		c.Fatal("Expected not to have containers", out)
	}
}

func (s *DockerSuite) TestRunPIDHostWithChildIsKillable(c *testing.T) {
	// Not applicable on Windows as uses Unix specific functionality
	testRequires(c, DaemonIsLinux, NotUserNamespace)
	name := "ibuildthecloud"
	dockerCmd(c, "run", "-d", "--pid=host", "--name", name, "busybox", "sh", "-c", "sleep 30; echo hi")

	assert.Assert(c, waitRun(name) == nil)

	errchan := make(chan error, 1)
	go func() {
		if out, _, err := dockerCmdWithError("kill", name); err != nil {
			errchan <- fmt.Errorf("%v:\n%s", err, out)
		}
		close(errchan)
	}()
	select {
	case err := <-errchan:
		assert.NilError(c, err)
	case <-time.After(5 * time.Second):
		c.Fatal("Kill container timed out")
	}
}

func (s *DockerSuite) TestRunWithTooSmallMemoryLimit(c *testing.T) {
	// TODO Windows. This may be possible to enable once Windows supports memory limits on containers
	testRequires(c, DaemonIsLinux)
	// this memory limit is 1 byte less than the min (daemon.linuxMinMemory), which is 6MB (6291456 bytes)
	out, _, err := dockerCmdWithError("create", "-m", "6291455", "busybox")
	if err == nil || !strings.Contains(out, "Minimum memory limit allowed is 6MB") {
		c.Fatalf("expected run to fail when using too low a memory limit: %q", out)
	}
}

func (s *DockerSuite) TestRunWriteToProcAsound(c *testing.T) {
	// Not applicable on Windows as uses Unix specific functionality
	testRequires(c, DaemonIsLinux)
	_, code, err := dockerCmdWithError("run", "busybox", "sh", "-c", "echo 111 >> /proc/asound/version")
	if err == nil || code == 0 {
		c.Fatal("standard container should not be able to write to /proc/asound")
	}
}

func (s *DockerSuite) TestRunReadProcTimer(c *testing.T) {
	// Not applicable on Windows as uses Unix specific functionality
	testRequires(c, DaemonIsLinux)
	out, code, err := dockerCmdWithError("run", "busybox", "cat", "/proc/timer_stats")
	if code != 0 {
		return
	}
	if err != nil {
		c.Fatal(err)
	}
	if strings.Trim(out, "\n ") != "" {
		c.Fatalf("expected to receive no output from /proc/timer_stats but received %q", out)
	}
}

func (s *DockerSuite) TestRunReadProcLatency(c *testing.T) {
	// Not applicable on Windows as uses Unix specific functionality
	testRequires(c, DaemonIsLinux)
	// some kernels don't have this configured so skip the test if this file is not found
	// on the host running the tests.
	if _, err := os.Stat("/proc/latency_stats"); err != nil {
		c.Skip("kernel doesn't have latency_stats configured")
		return
	}
	out, code, err := dockerCmdWithError("run", "busybox", "cat", "/proc/latency_stats")
	if code != 0 {
		return
	}
	if err != nil {
		c.Fatal(err)
	}
	if strings.Trim(out, "\n ") != "" {
		c.Fatalf("expected to receive no output from /proc/latency_stats but received %q", out)
	}
}

func (s *DockerSuite) TestRunReadFilteredProc(c *testing.T) {
	// Not applicable on Windows as uses Unix specific functionality
	testRequires(c, Apparmor, DaemonIsLinux, NotUserNamespace)

	testReadPaths := []string{
		"/proc/latency_stats",
		"/proc/timer_stats",
		"/proc/kcore",
	}
	for i, filePath := range testReadPaths {
		name := fmt.Sprintf("procsieve-%d", i)
		shellCmd := fmt.Sprintf("exec 3<%s", filePath)

		out, exitCode, err := dockerCmdWithError("run", "--privileged", "--security-opt", "apparmor=docker-default", "--name", name, "busybox", "sh", "-c", shellCmd)
		if exitCode != 0 {
			return
		}
		if err != nil {
			c.Fatalf("Open FD for read should have failed with permission denied, got: %s, %v", out, err)
		}
	}
}

func (s *DockerSuite) TestMountIntoProc(c *testing.T) {
	// Not applicable on Windows as uses Unix specific functionality
	testRequires(c, DaemonIsLinux)
	_, code, err := dockerCmdWithError("run", "-v", "/proc//sys", "busybox", "true")
	if err == nil || code == 0 {
		c.Fatal("container should not be able to mount into /proc")
	}
}

func (s *DockerSuite) TestMountIntoSys(c *testing.T) {
	// Not applicable on Windows as uses Unix specific functionality
	testRequires(c, DaemonIsLinux)
	testRequires(c, NotUserNamespace)
	dockerCmd(c, "run", "-v", "/sys/fs/cgroup", "busybox", "true")
}

func (s *DockerSuite) TestRunUnshareProc(c *testing.T) {
	// Not applicable on Windows as uses Unix specific functionality
	testRequires(c, Apparmor, DaemonIsLinux, NotUserNamespace)

	// In this test goroutines are used to run test cases in parallel to prevent the test from taking a long time to run.
	errChan := make(chan error)

	go func() {
		name := "acidburn"
		out, _, err := dockerCmdWithError("run", "--name", name, "--security-opt", "seccomp=unconfined", "debian:buster", "unshare", "-p", "-m", "-f", "-r", "--mount-proc=/proc", "mount")
		if err == nil ||
			!(strings.Contains(strings.ToLower(out), "permission denied") ||
				strings.Contains(strings.ToLower(out), "operation not permitted")) {
			errChan <- fmt.Errorf("unshare with --mount-proc should have failed with 'permission denied' or 'operation not permitted', got: %s, %v", out, err)
		} else {
			errChan <- nil
		}
	}()

	go func() {
		name := "cereal"
		out, _, err := dockerCmdWithError("run", "--name", name, "--security-opt", "seccomp=unconfined", "debian:buster", "unshare", "-p", "-m", "-f", "-r", "mount", "-t", "proc", "none", "/proc")
		if err == nil ||
			!(strings.Contains(strings.ToLower(out), "mount: cannot mount none") ||
				strings.Contains(strings.ToLower(out), "permission denied") ||
				strings.Contains(strings.ToLower(out), "operation not permitted")) {
			errChan <- fmt.Errorf("unshare and mount of /proc should have failed with 'mount: cannot mount none' or 'permission denied', got: %s, %v", out, err)
		} else {
			errChan <- nil
		}
	}()

	/* Ensure still fails if running privileged with the default policy */
	go func() {
		name := "crashoverride"
		out, _, err := dockerCmdWithError("run", "--privileged", "--security-opt", "seccomp=unconfined", "--security-opt", "apparmor=docker-default", "--name", name, "debian:buster", "unshare", "-p", "-m", "-f", "-r", "mount", "-t", "proc", "none", "/proc")
		if err == nil ||
			!(strings.Contains(strings.ToLower(out), "mount: cannot mount none") ||
				strings.Contains(strings.ToLower(out), "permission denied") ||
				strings.Contains(strings.ToLower(out), "operation not permitted")) {
			errChan <- fmt.Errorf("privileged unshare with apparmor should have failed with 'mount: cannot mount none' or 'permission denied', got: %s, %v", out, err)
		} else {
			errChan <- nil
		}
	}()

	var retErr error
	for i := 0; i < 3; i++ {
		err := <-errChan
		if retErr == nil && err != nil {
			retErr = err
		}
	}
	if retErr != nil {
		c.Fatal(retErr)
	}
}

func (s *DockerSuite) TestRunPublishPort(c *testing.T) {
	// TODO Windows: This may be possible once Windows moves to libnetwork and CNM
	testRequires(c, DaemonIsLinux)
	dockerCmd(c, "run", "-d", "--name", "test", "--expose", "8080", "busybox", "top")
	out, _ := dockerCmd(c, "port", "test")
	out = strings.Trim(out, "\r\n")
	if out != "" {
		c.Fatalf("run without --publish-all should not publish port, out should be nil, but got: %s", out)
	}
}

// Issue #10184.
func (s *DockerSuite) TestDevicePermissions(c *testing.T) {
	// Not applicable on Windows as uses Unix specific functionality
	testRequires(c, DaemonIsLinux)
	const permissions = "crw-rw-rw-"
	out, status := dockerCmd(c, "run", "--device", "/dev/fuse:/dev/fuse:mrw", "busybox:latest", "ls", "-l", "/dev/fuse")
	if status != 0 {
		c.Fatalf("expected status 0, got %d", status)
	}
	if !strings.HasPrefix(out, permissions) {
		c.Fatalf("output should begin with %q, got %q", permissions, out)
	}
}

func (s *DockerSuite) TestRunCapAddCHOWN(c *testing.T) {
	// Not applicable on Windows as uses Unix specific functionality
	testRequires(c, DaemonIsLinux)
	out, _ := dockerCmd(c, "run", "--cap-drop=ALL", "--cap-add=CHOWN", "busybox", "sh", "-c", "adduser -D -H newuser && chown newuser /home && echo ok")

	if actual := strings.Trim(out, "\r\n"); actual != "ok" {
		c.Fatalf("expected output ok received %s", actual)
	}
}

// https://github.com/docker/docker/pull/14498
func (s *DockerSuite) TestVolumeFromMixedRWOptions(c *testing.T) {
	prefix, slash := getPrefixAndSlashFromDaemonPlatform()

	dockerCmd(c, "run", "--name", "parent", "-v", prefix+"/test", "busybox", "true")

	dockerCmd(c, "run", "--volumes-from", "parent:ro", "--name", "test-volumes-1", "busybox", "true")
	dockerCmd(c, "run", "--volumes-from", "parent:rw", "--name", "test-volumes-2", "busybox", "true")

	if testEnv.OSType != "windows" {
		mRO, err := inspectMountPoint("test-volumes-1", prefix+slash+"test")
		assert.NilError(c, err, "failed to inspect mount point")
		if mRO.RW {
			c.Fatalf("Expected RO volume was RW")
		}
	}

	mRW, err := inspectMountPoint("test-volumes-2", prefix+slash+"test")
	assert.NilError(c, err, "failed to inspect mount point")
	if !mRW.RW {
		c.Fatalf("Expected RW volume was RO")
	}
}

func (s *DockerSuite) TestRunWriteFilteredProc(c *testing.T) {
	// Not applicable on Windows as uses Unix specific functionality
	testRequires(c, Apparmor, DaemonIsLinux, NotUserNamespace)

	testWritePaths := []string{
		/* modprobe and core_pattern should both be denied by generic
		 * policy of denials for /proc/sys/kernel. These files have been
		 * picked to be checked as they are particularly sensitive to writes */
		"/proc/sys/kernel/modprobe",
		"/proc/sys/kernel/core_pattern",
		"/proc/sysrq-trigger",
		"/proc/kcore",
	}
	for i, filePath := range testWritePaths {
		name := fmt.Sprintf("writeprocsieve-%d", i)

		shellCmd := fmt.Sprintf("exec 3>%s", filePath)
		out, code, err := dockerCmdWithError("run", "--privileged", "--security-opt", "apparmor=docker-default", "--name", name, "busybox", "sh", "-c", shellCmd)
		if code != 0 {
			return
		}
		if err != nil {
			c.Fatalf("Open FD for write should have failed with permission denied, got: %s, %v", out, err)
		}
	}
}

func (s *DockerSuite) TestRunNetworkFilesBindMount(c *testing.T) {
	// Not applicable on Windows as uses Unix specific functionality
	testRequires(c, testEnv.IsLocalDaemon, DaemonIsLinux)

	expected := "test123"

	filename := createTmpFile(c, expected)
	defer os.Remove(filename)

	// for user namespaced test runs, the temp file must be accessible to unprivileged root
	if err := os.Chmod(filename, 0646); err != nil {
		c.Fatalf("error modifying permissions of %s: %v", filename, err)
	}

	nwfiles := []string{"/etc/resolv.conf", "/etc/hosts", "/etc/hostname"}

	for i := range nwfiles {
		actual, _ := dockerCmd(c, "run", "-v", filename+":"+nwfiles[i], "busybox", "cat", nwfiles[i])
		if actual != expected {
			c.Fatalf("expected %s be: %q, but was: %q", nwfiles[i], expected, actual)
		}
	}
}

func (s *DockerSuite) TestRunNetworkFilesBindMountRO(c *testing.T) {
	// Not applicable on Windows as uses Unix specific functionality
	testRequires(c, testEnv.IsLocalDaemon, DaemonIsLinux)

	filename := createTmpFile(c, "test123")
	defer os.Remove(filename)

	// for user namespaced test runs, the temp file must be accessible to unprivileged root
	if err := os.Chmod(filename, 0646); err != nil {
		c.Fatalf("error modifying permissions of %s: %v", filename, err)
	}

	nwfiles := []string{"/etc/resolv.conf", "/etc/hosts", "/etc/hostname"}

	for i := range nwfiles {
		_, exitCode, err := dockerCmdWithError("run", "-v", filename+":"+nwfiles[i]+":ro", "busybox", "touch", nwfiles[i])
		if err == nil || exitCode == 0 {
			c.Fatalf("run should fail because bind mount of %s is ro: exit code %d", nwfiles[i], exitCode)
		}
	}
}

func (s *DockerSuite) TestRunNetworkFilesBindMountROFilesystem(c *testing.T) {
	// Not applicable on Windows as uses Unix specific functionality
	testRequires(c, testEnv.IsLocalDaemon, DaemonIsLinux, UserNamespaceROMount)

	filename := createTmpFile(c, "test123")
	defer os.Remove(filename)

	// for user namespaced test runs, the temp file must be accessible to unprivileged root
	if err := os.Chmod(filename, 0646); err != nil {
		c.Fatalf("error modifying permissions of %s: %v", filename, err)
	}

	nwfiles := []string{"/etc/resolv.conf", "/etc/hosts", "/etc/hostname"}

	for i := range nwfiles {
		_, exitCode := dockerCmd(c, "run", "-v", filename+":"+nwfiles[i], "--read-only", "busybox", "touch", nwfiles[i])
		if exitCode != 0 {
			c.Fatalf("run should not fail because %s is mounted writable on read-only root filesystem: exit code %d", nwfiles[i], exitCode)
		}
	}

	for i := range nwfiles {
		_, exitCode, err := dockerCmdWithError("run", "-v", filename+":"+nwfiles[i]+":ro", "--read-only", "busybox", "touch", nwfiles[i])
		if err == nil || exitCode == 0 {
			c.Fatalf("run should fail because %s is mounted read-only on read-only root filesystem: exit code %d", nwfiles[i], exitCode)
		}
	}
}

func (s *DockerSuite) TestPtraceContainerProcsFromHost(c *testing.T) {
	// Not applicable on Windows as uses Unix specific functionality
	testRequires(c, DaemonIsLinux, testEnv.IsLocalDaemon)

	out, _ := dockerCmd(c, "run", "-d", "busybox", "top")
	id := strings.TrimSpace(out)
	assert.NilError(c, waitRun(id))
	pid1 := inspectField(c, id, "State.Pid")

	_, err := os.Readlink(fmt.Sprintf("/proc/%s/ns/net", pid1))
	if err != nil {
		c.Fatal(err)
	}
}

func (s *DockerSuite) TestAppArmorDeniesPtrace(c *testing.T) {
	// Not applicable on Windows as uses Unix specific functionality
	testRequires(c, testEnv.IsLocalDaemon, Apparmor, DaemonIsLinux)

	// Run through 'sh' so we are NOT pid 1. Pid 1 may be able to trace
	// itself, but pid>1 should not be able to trace pid1.
	_, exitCode, _ := dockerCmdWithError("run", "busybox", "sh", "-c", "sh -c readlink /proc/1/ns/net")
	if exitCode == 0 {
		c.Fatal("ptrace was not successfully restricted by AppArmor")
	}
}

func (s *DockerSuite) TestAppArmorTraceSelf(c *testing.T) {
	// Not applicable on Windows as uses Unix specific functionality
	testRequires(c, DaemonIsLinux, testEnv.IsLocalDaemon, Apparmor)

	_, exitCode, _ := dockerCmdWithError("run", "busybox", "readlink", "/proc/1/ns/net")
	if exitCode != 0 {
		c.Fatal("ptrace of self failed.")
	}
}

func (s *DockerSuite) TestAppArmorDeniesChmodProc(c *testing.T) {
	// Not applicable on Windows as uses Unix specific functionality
	testRequires(c, testEnv.IsLocalDaemon, Apparmor, DaemonIsLinux, NotUserNamespace)
	_, exitCode, _ := dockerCmdWithError("run", "busybox", "chmod", "744", "/proc/cpuinfo")
	if exitCode == 0 {
		// If our test failed, attempt to repair the host system...
		_, exitCode, _ := dockerCmdWithError("run", "busybox", "chmod", "444", "/proc/cpuinfo")
		if exitCode == 0 {
			c.Fatal("AppArmor was unsuccessful in prohibiting chmod of /proc/* files.")
		}
	}
}

func (s *DockerSuite) TestRunCapAddSYSTIME(c *testing.T) {
	// Not applicable on Windows as uses Unix specific functionality
	testRequires(c, DaemonIsLinux)

	dockerCmd(c, "run", "--cap-drop=ALL", "--cap-add=SYS_TIME", "busybox", "sh", "-c", "grep ^CapEff /proc/self/status | sed 's/^CapEff:\t//' | grep ^0000000002000000$")
}

// run create container failed should clean up the container
func (s *DockerSuite) TestRunCreateContainerFailedCleanUp(c *testing.T) {
	// TODO Windows. This may be possible to enable once link is supported
	testRequires(c, DaemonIsLinux)
	name := "unique_name"
	_, _, err := dockerCmdWithError("run", "--name", name, "--link", "nothing:nothing", "busybox")
	assert.Assert(c, err != nil, "Expected docker run to fail!")

	containerID, err := inspectFieldWithError(name, "Id")
	assert.Assert(c, err != nil, "Expected not to have this container: %s!", containerID)
	assert.Equal(c, containerID, "", fmt.Sprintf("Expected not to have this container: %s!", containerID))
}

func (s *DockerSuite) TestRunNamedVolume(c *testing.T) {
	prefix, _ := getPrefixAndSlashFromDaemonPlatform()
	testRequires(c, DaemonIsLinux)
	dockerCmd(c, "run", "--name=test", "-v", "testing:"+prefix+"/foo", "busybox", "sh", "-c", "echo hello > "+prefix+"/foo/bar")

	out, _ := dockerCmd(c, "run", "--volumes-from", "test", "busybox", "sh", "-c", "cat "+prefix+"/foo/bar")
	assert.Equal(c, strings.TrimSpace(out), "hello")

	out, _ = dockerCmd(c, "run", "-v", "testing:"+prefix+"/foo", "busybox", "sh", "-c", "cat "+prefix+"/foo/bar")
	assert.Equal(c, strings.TrimSpace(out), "hello")
}

func (s *DockerSuite) TestRunWithUlimits(c *testing.T) {
	// Not applicable on Windows as uses Unix specific functionality
	testRequires(c, DaemonIsLinux)

	out, _ := dockerCmd(c, "run", "--name=testulimits", "--ulimit", "nofile=42", "busybox", "/bin/sh", "-c", "ulimit -n")
	ul := strings.TrimSpace(out)
	if ul != "42" {
		c.Fatalf("expected `ulimit -n` to be 42, got %s", ul)
	}
}

func (s *DockerSuite) TestRunContainerWithCgroupParent(c *testing.T) {
	// Not applicable on Windows as uses Unix specific functionality
	testRequires(c, DaemonIsLinux)

	// cgroup-parent relative path
	testRunContainerWithCgroupParent(c, "test", "cgroup-test")

	// cgroup-parent absolute path
	testRunContainerWithCgroupParent(c, "/cgroup-parent/test", "cgroup-test-absolute")
}

func testRunContainerWithCgroupParent(c *testing.T, cgroupParent, name string) {
	out, _, err := dockerCmdWithError("run", "--cgroup-parent", cgroupParent, "--name", name, "busybox", "cat", "/proc/self/cgroup")
	if err != nil {
		c.Fatalf("unexpected failure when running container with --cgroup-parent option - %s\n%v", out, err)
	}
	cgroupPaths := ParseCgroupPaths(out)
	if len(cgroupPaths) == 0 {
		c.Fatalf("unexpected output - %q", out)
	}
	id := getIDByName(c, name)
	expectedCgroup := path.Join(cgroupParent, id)
	found := false
	for _, path := range cgroupPaths {
		if strings.HasSuffix(path, expectedCgroup) {
			found = true
			break
		}
	}
	if !found {
		c.Fatalf("unexpected cgroup paths. Expected at least one cgroup path to have suffix %q. Cgroup Paths: %v", expectedCgroup, cgroupPaths)
	}
}

// TestRunInvalidCgroupParent checks that a specially-crafted cgroup parent doesn't cause Docker to crash or start modifying /.
func (s *DockerSuite) TestRunInvalidCgroupParent(c *testing.T) {
	// Not applicable on Windows as uses Unix specific functionality
	testRequires(c, DaemonIsLinux)

	testRunInvalidCgroupParent(c, "../../../../../../../../SHOULD_NOT_EXIST", "SHOULD_NOT_EXIST", "cgroup-invalid-test")

	testRunInvalidCgroupParent(c, "/../../../../../../../../SHOULD_NOT_EXIST", "/SHOULD_NOT_EXIST", "cgroup-absolute-invalid-test")
}

func testRunInvalidCgroupParent(c *testing.T, cgroupParent, cleanCgroupParent, name string) {
	out, _, err := dockerCmdWithError("run", "--cgroup-parent", cgroupParent, "--name", name, "busybox", "cat", "/proc/self/cgroup")
	if err != nil {
		// XXX: This may include a daemon crash.
		c.Fatalf("unexpected failure when running container with --cgroup-parent option - %s\n%v", out, err)
	}

	// We expect "/SHOULD_NOT_EXIST" to not exist. If not, we have a security issue.
	if _, err := os.Stat("/SHOULD_NOT_EXIST"); err == nil || !os.IsNotExist(err) {
		c.Fatalf("SECURITY: --cgroup-parent with ../../ relative paths cause files to be created in the host (this is bad) !!")
	}

	cgroupPaths := ParseCgroupPaths(out)
	if len(cgroupPaths) == 0 {
		c.Fatalf("unexpected output - %q", out)
	}
	id := getIDByName(c, name)
	expectedCgroup := path.Join(cleanCgroupParent, id)
	found := false
	for _, path := range cgroupPaths {
		if strings.HasSuffix(path, expectedCgroup) {
			found = true
			break
		}
	}
	if !found {
		c.Fatalf("unexpected cgroup paths. Expected at least one cgroup path to have suffix %q. Cgroup Paths: %v", expectedCgroup, cgroupPaths)
	}
}

func (s *DockerSuite) TestRunContainerWithCgroupMountRO(c *testing.T) {
	// Not applicable on Windows as uses Unix specific functionality
	// --read-only + userns has remount issues
	testRequires(c, DaemonIsLinux, NotUserNamespace)

	filename := "/sys/fs/cgroup/devices/test123"
	out, _, err := dockerCmdWithError("run", "busybox", "touch", filename)
	if err == nil {
		c.Fatal("expected cgroup mount point to be read-only, touch file should fail")
	}
	expected := "Read-only file system"
	if !strings.Contains(out, expected) {
		c.Fatalf("expected output from failure to contain %s but contains %s", expected, out)
	}
}

func (s *DockerSuite) TestRunContainerNetworkModeToSelf(c *testing.T) {
	// Not applicable on Windows which does not support --net=container
	testRequires(c, DaemonIsLinux)
	out, _, err := dockerCmdWithError("run", "--name=me", "--net=container:me", "busybox", "true")
	if err == nil || !strings.Contains(out, "cannot join own network") {
		c.Fatalf("using container net mode to self should result in an error\nerr: %q\nout: %s", err, out)
	}
}

func (s *DockerSuite) TestRunContainerNetModeWithDNSMacHosts(c *testing.T) {
	// Not applicable on Windows which does not support --net=container
	testRequires(c, DaemonIsLinux)
	out, _, err := dockerCmdWithError("run", "-d", "--name", "parent", "busybox", "top")
	if err != nil {
		c.Fatalf("failed to run container: %v, output: %q", err, out)
	}

	out, _, err = dockerCmdWithError("run", "--dns", "1.2.3.4", "--net=container:parent", "busybox")
	if err == nil || !strings.Contains(out, runconfig.ErrConflictNetworkAndDNS.Error()) {
		c.Fatalf("run --net=container with --dns should error out")
	}

	out, _, err = dockerCmdWithError("run", "--mac-address", "92:d0:c6:0a:29:33", "--net=container:parent", "busybox")
	if err == nil || !strings.Contains(out, runconfig.ErrConflictContainerNetworkAndMac.Error()) {
		c.Fatalf("run --net=container with --mac-address should error out")
	}

	out, _, err = dockerCmdWithError("run", "--add-host", "test:192.168.2.109", "--net=container:parent", "busybox")
	if err == nil || !strings.Contains(out, runconfig.ErrConflictNetworkHosts.Error()) {
		c.Fatalf("run --net=container with --add-host should error out")
	}
}

func (s *DockerSuite) TestRunContainerNetModeWithExposePort(c *testing.T) {
	// Not applicable on Windows which does not support --net=container
	testRequires(c, DaemonIsLinux)
	dockerCmd(c, "run", "-d", "--name", "parent", "busybox", "top")

	out, _, err := dockerCmdWithError("run", "-p", "5000:5000", "--net=container:parent", "busybox")
	if err == nil || !strings.Contains(out, runconfig.ErrConflictNetworkPublishPorts.Error()) {
		c.Fatalf("run --net=container with -p should error out")
	}

	out, _, err = dockerCmdWithError("run", "-P", "--net=container:parent", "busybox")
	if err == nil || !strings.Contains(out, runconfig.ErrConflictNetworkPublishPorts.Error()) {
		c.Fatalf("run --net=container with -P should error out")
	}

	out, _, err = dockerCmdWithError("run", "--expose", "5000", "--net=container:parent", "busybox")
	if err == nil || !strings.Contains(out, runconfig.ErrConflictNetworkExposePorts.Error()) {
		c.Fatalf("run --net=container with --expose should error out")
	}
}

func (s *DockerSuite) TestRunLinkToContainerNetMode(c *testing.T) {
	// Not applicable on Windows which does not support --net=container or --link
	testRequires(c, DaemonIsLinux)
	dockerCmd(c, "run", "--name", "test", "-d", "busybox", "top")
	dockerCmd(c, "run", "--name", "parent", "-d", "--net=container:test", "busybox", "top")
	dockerCmd(c, "run", "-d", "--link=parent:parent", "busybox", "top")
	dockerCmd(c, "run", "--name", "child", "-d", "--net=container:parent", "busybox", "top")
	dockerCmd(c, "run", "-d", "--link=child:child", "busybox", "top")
}

func (s *DockerSuite) TestRunLoopbackOnlyExistsWhenNetworkingDisabled(c *testing.T) {
	// TODO Windows: This may be possible to convert.
	testRequires(c, DaemonIsLinux)
	out, _ := dockerCmd(c, "run", "--net=none", "busybox", "ip", "-o", "-4", "a", "show", "up")

	var (
		count = 0
		parts = strings.Split(out, "\n")
	)

	for _, l := range parts {
		if l != "" {
			count++
		}
	}

	if count != 1 {
		c.Fatalf("Wrong interface count in container %d", count)
	}

	if !strings.HasPrefix(out, "1: lo") {
		c.Fatalf("Wrong interface in test container: expected [1: lo], got %s", out)
	}
}

// Issue #4681
func (s *DockerSuite) TestRunLoopbackWhenNetworkDisabled(c *testing.T) {
	if testEnv.OSType == "windows" {
		dockerCmd(c, "run", "--net=none", testEnv.PlatformDefaults.BaseImage, "ping", "-n", "1", "127.0.0.1")
	} else {
		dockerCmd(c, "run", "--net=none", "busybox", "ping", "-c", "1", "127.0.0.1")
	}
}

func (s *DockerSuite) TestRunModeNetContainerHostname(c *testing.T) {
	// Windows does not support --net=container
	testRequires(c, DaemonIsLinux)

	dockerCmd(c, "run", "-i", "-d", "--name", "parent", "busybox", "top")
	out, _ := dockerCmd(c, "exec", "parent", "cat", "/etc/hostname")
	out1, _ := dockerCmd(c, "run", "--net=container:parent", "busybox", "cat", "/etc/hostname")

	if out1 != out {
		c.Fatal("containers with shared net namespace should have same hostname")
	}
}

func (s *DockerSuite) TestRunNetworkNotInitializedNoneMode(c *testing.T) {
	// TODO Windows: Network settings are not currently propagated. This may
	// be resolved in the future with the move to libnetwork and CNM.
	testRequires(c, DaemonIsLinux)
	out, _ := dockerCmd(c, "run", "-d", "--net=none", "busybox", "top")
	id := strings.TrimSpace(out)
	res := inspectField(c, id, "NetworkSettings.Networks.none.IPAddress")
	if res != "" {
		c.Fatalf("For 'none' mode network must not be initialized, but container got IP: %s", res)
	}
}

func (s *DockerSuite) TestTwoContainersInNetHost(c *testing.T) {
	// Not applicable as Windows does not support --net=host
	testRequires(c, DaemonIsLinux, NotUserNamespace, NotUserNamespace)
	dockerCmd(c, "run", "-d", "--net=host", "--name=first", "busybox", "top")
	dockerCmd(c, "run", "-d", "--net=host", "--name=second", "busybox", "top")
	dockerCmd(c, "stop", "first")
	dockerCmd(c, "stop", "second")
}

func (s *DockerSuite) TestContainersInUserDefinedNetwork(c *testing.T) {
	testRequires(c, DaemonIsLinux, NotUserNamespace, NotArm)
	dockerCmd(c, "network", "create", "-d", "bridge", "testnetwork")
	dockerCmd(c, "run", "-d", "--net=testnetwork", "--name=first", "busybox", "top")
	assert.Assert(c, waitRun("first") == nil)
	dockerCmd(c, "run", "-t", "--net=testnetwork", "--name=second", "busybox", "ping", "-c", "1", "first")
}

func (s *DockerSuite) TestContainersInMultipleNetworks(c *testing.T) {
	testRequires(c, DaemonIsLinux, NotUserNamespace, NotArm)
	// Create 2 networks using bridge driver
	dockerCmd(c, "network", "create", "-d", "bridge", "testnetwork1")
	dockerCmd(c, "network", "create", "-d", "bridge", "testnetwork2")
	// Run and connect containers to testnetwork1
	dockerCmd(c, "run", "-d", "--net=testnetwork1", "--name=first", "busybox", "top")
	assert.Assert(c, waitRun("first") == nil)
	dockerCmd(c, "run", "-d", "--net=testnetwork1", "--name=second", "busybox", "top")
	assert.Assert(c, waitRun("second") == nil)
	// Check connectivity between containers in testnetwork2
	dockerCmd(c, "exec", "first", "ping", "-c", "1", "second.testnetwork1")
	// Connect containers to testnetwork2
	dockerCmd(c, "network", "connect", "testnetwork2", "first")
	dockerCmd(c, "network", "connect", "testnetwork2", "second")
	// Check connectivity between containers
	dockerCmd(c, "exec", "second", "ping", "-c", "1", "first.testnetwork2")
}

func (s *DockerSuite) TestContainersNetworkIsolation(c *testing.T) {
	testRequires(c, DaemonIsLinux, NotUserNamespace, NotArm)
	// Create 2 networks using bridge driver
	dockerCmd(c, "network", "create", "-d", "bridge", "testnetwork1")
	dockerCmd(c, "network", "create", "-d", "bridge", "testnetwork2")
	// Run 1 container in testnetwork1 and another in testnetwork2
	dockerCmd(c, "run", "-d", "--net=testnetwork1", "--name=first", "busybox", "top")
	assert.Assert(c, waitRun("first") == nil)
	dockerCmd(c, "run", "-d", "--net=testnetwork2", "--name=second", "busybox", "top")
	assert.Assert(c, waitRun("second") == nil)

	// Check Isolation between containers : ping must fail
	_, _, err := dockerCmdWithError("exec", "first", "ping", "-c", "1", "second")
	assert.ErrorContains(c, err, "")
	// Connect first container to testnetwork2
	dockerCmd(c, "network", "connect", "testnetwork2", "first")
	// ping must succeed now
	_, _, err = dockerCmdWithError("exec", "first", "ping", "-c", "1", "second")
	assert.NilError(c, err)

	// Disconnect first container from testnetwork2
	dockerCmd(c, "network", "disconnect", "testnetwork2", "first")
	// ping must fail again
	_, _, err = dockerCmdWithError("exec", "first", "ping", "-c", "1", "second")
	assert.ErrorContains(c, err, "")
}

func (s *DockerSuite) TestNetworkRmWithActiveContainers(c *testing.T) {
	testRequires(c, DaemonIsLinux, NotUserNamespace)
	// Create 2 networks using bridge driver
	dockerCmd(c, "network", "create", "-d", "bridge", "testnetwork1")
	// Run and connect containers to testnetwork1
	dockerCmd(c, "run", "-d", "--net=testnetwork1", "--name=first", "busybox", "top")
	assert.Assert(c, waitRun("first") == nil)
	dockerCmd(c, "run", "-d", "--net=testnetwork1", "--name=second", "busybox", "top")
	assert.Assert(c, waitRun("second") == nil)
	// Network delete with active containers must fail
	_, _, err := dockerCmdWithError("network", "rm", "testnetwork1")
	assert.ErrorContains(c, err, "")

	dockerCmd(c, "stop", "first")
	_, _, err = dockerCmdWithError("network", "rm", "testnetwork1")
	assert.ErrorContains(c, err, "")
}

func (s *DockerSuite) TestContainerRestartInMultipleNetworks(c *testing.T) {
	testRequires(c, DaemonIsLinux, NotUserNamespace, NotArm)
	// Create 2 networks using bridge driver
	dockerCmd(c, "network", "create", "-d", "bridge", "testnetwork1")
	dockerCmd(c, "network", "create", "-d", "bridge", "testnetwork2")

	// Run and connect containers to testnetwork1
	dockerCmd(c, "run", "-d", "--net=testnetwork1", "--name=first", "busybox", "top")
	assert.Assert(c, waitRun("first") == nil)
	dockerCmd(c, "run", "-d", "--net=testnetwork1", "--name=second", "busybox", "top")
	assert.Assert(c, waitRun("second") == nil)
	// Check connectivity between containers in testnetwork2
	dockerCmd(c, "exec", "first", "ping", "-c", "1", "second.testnetwork1")
	// Connect containers to testnetwork2
	dockerCmd(c, "network", "connect", "testnetwork2", "first")
	dockerCmd(c, "network", "connect", "testnetwork2", "second")
	// Check connectivity between containers
	dockerCmd(c, "exec", "second", "ping", "-c", "1", "first.testnetwork2")

	// Stop second container and test ping failures on both networks
	dockerCmd(c, "stop", "second")
	_, _, err := dockerCmdWithError("exec", "first", "ping", "-c", "1", "second.testnetwork1")
	assert.ErrorContains(c, err, "")
	_, _, err = dockerCmdWithError("exec", "first", "ping", "-c", "1", "second.testnetwork2")
	assert.ErrorContains(c, err, "")

	// Start second container and connectivity must be restored on both networks
	dockerCmd(c, "start", "second")
	dockerCmd(c, "exec", "first", "ping", "-c", "1", "second.testnetwork1")
	dockerCmd(c, "exec", "second", "ping", "-c", "1", "first.testnetwork2")
}

func (s *DockerSuite) TestContainerWithConflictingHostNetworks(c *testing.T) {
	testRequires(c, DaemonIsLinux, NotUserNamespace)
	// Run a container with --net=host
	dockerCmd(c, "run", "-d", "--net=host", "--name=first", "busybox", "top")
	assert.Assert(c, waitRun("first") == nil)

	// Create a network using bridge driver
	dockerCmd(c, "network", "create", "-d", "bridge", "testnetwork1")

	// Connecting to the user defined network must fail
	_, _, err := dockerCmdWithError("network", "connect", "testnetwork1", "first")
	assert.ErrorContains(c, err, "")
}

func (s *DockerSuite) TestContainerWithConflictingSharedNetwork(c *testing.T) {
	testRequires(c, DaemonIsLinux)
	dockerCmd(c, "run", "-d", "--name=first", "busybox", "top")
	assert.Assert(c, waitRun("first") == nil)
	// Run second container in first container's network namespace
	dockerCmd(c, "run", "-d", "--net=container:first", "--name=second", "busybox", "top")
	assert.Assert(c, waitRun("second") == nil)

	// Create a network using bridge driver
	dockerCmd(c, "network", "create", "-d", "bridge", "testnetwork1")

	// Connecting to the user defined network must fail
	out, _, err := dockerCmdWithError("network", "connect", "testnetwork1", "second")
	assert.ErrorContains(c, err, "")
	assert.Assert(c, strings.Contains(out, runconfig.ErrConflictSharedNetwork.Error()))
}

func (s *DockerSuite) TestContainerWithConflictingNoneNetwork(c *testing.T) {
	testRequires(c, DaemonIsLinux)
	dockerCmd(c, "run", "-d", "--net=none", "--name=first", "busybox", "top")
	assert.Assert(c, waitRun("first") == nil)

	// Create a network using bridge driver
	dockerCmd(c, "network", "create", "-d", "bridge", "testnetwork1")

	// Connecting to the user defined network must fail
	out, _, err := dockerCmdWithError("network", "connect", "testnetwork1", "first")
	assert.ErrorContains(c, err, "")
	assert.Assert(c, strings.Contains(out, runconfig.ErrConflictNoNetwork.Error()))
	// create a container connected to testnetwork1
	dockerCmd(c, "run", "-d", "--net=testnetwork1", "--name=second", "busybox", "top")
	assert.Assert(c, waitRun("second") == nil)

	// Connect second container to none network. it must fail as well
	_, _, err = dockerCmdWithError("network", "connect", "none", "second")
	assert.ErrorContains(c, err, "")
}

// #11957 - stdin with no tty does not exit if stdin is not closed even though container exited
func (s *DockerSuite) TestRunStdinBlockedAfterContainerExit(c *testing.T) {
	cmd := exec.Command(dockerBinary, "run", "-i", "--name=test", "busybox", "true")
	in, err := cmd.StdinPipe()
	assert.NilError(c, err)
	defer in.Close()
	stdout := bytes.NewBuffer(nil)
	cmd.Stdout = stdout
	cmd.Stderr = stdout
	assert.Assert(c, cmd.Start() == nil)

	waitChan := make(chan error, 1)
	go func() {
		waitChan <- cmd.Wait()
	}()

	select {
	case err := <-waitChan:
		assert.Assert(c, err == nil, stdout.String())
	case <-time.After(30 * time.Second):
		c.Fatal("timeout waiting for command to exit")
	}
}

func (s *DockerSuite) TestRunWrongCpusetCpusFlagValue(c *testing.T) {
	// TODO Windows: This needs validation (error out) in the daemon.
	testRequires(c, DaemonIsLinux)
	out, exitCode, err := dockerCmdWithError("run", "--cpuset-cpus", "1-10,11--", "busybox", "true")
	assert.ErrorContains(c, err, "")
	expected := "Error response from daemon: Invalid value 1-10,11-- for cpuset cpus.\n"
	if !(strings.Contains(out, expected) || exitCode == 125) {
		c.Fatalf("Expected output to contain %q with exitCode 125, got out: %q exitCode: %v", expected, out, exitCode)
	}
}

func (s *DockerSuite) TestRunWrongCpusetMemsFlagValue(c *testing.T) {
	// TODO Windows: This needs validation (error out) in the daemon.
	testRequires(c, DaemonIsLinux)
	out, exitCode, err := dockerCmdWithError("run", "--cpuset-mems", "1-42--", "busybox", "true")
	assert.ErrorContains(c, err, "")
	expected := "Error response from daemon: Invalid value 1-42-- for cpuset mems.\n"
	if !(strings.Contains(out, expected) || exitCode == 125) {
		c.Fatalf("Expected output to contain %q with exitCode 125, got out: %q exitCode: %v", expected, out, exitCode)
	}
}

// TestRunNonExecutableCmd checks that 'docker run busybox foo' exits with error code 127'
func (s *DockerSuite) TestRunNonExecutableCmd(c *testing.T) {
	name := "testNonExecutableCmd"
	icmd.RunCommand(dockerBinary, "run", "--name", name, "busybox", "foo").Assert(c, icmd.Expected{
		ExitCode: 127,
		Error:    "exit status 127",
	})
}

// TestRunNonExistingCmd checks that 'docker run busybox /bin/foo' exits with code 127.
func (s *DockerSuite) TestRunNonExistingCmd(c *testing.T) {
	name := "testNonExistingCmd"
	icmd.RunCommand(dockerBinary, "run", "--name", name, "busybox", "/bin/foo").Assert(c, icmd.Expected{
		ExitCode: 127,
		Error:    "exit status 127",
	})
}

// TestCmdCannotBeInvoked checks that 'docker run busybox /etc' exits with 126, or
// 127 on Windows. The difference is that in Windows, the container must be started
// as that's when the check is made (and yes, by its design...)
func (s *DockerSuite) TestCmdCannotBeInvoked(c *testing.T) {
	expected := 126
	if testEnv.OSType == "windows" {
		expected = 127
	}
	name := "testCmdCannotBeInvoked"
	icmd.RunCommand(dockerBinary, "run", "--name", name, "busybox", "/etc").Assert(c, icmd.Expected{
		ExitCode: expected,
		Error:    fmt.Sprintf("exit status %d", expected),
	})
}

// TestRunNonExistingImage checks that 'docker run foo' exits with error msg 125 and contains  'Unable to find image'
// FIXME(vdemeester) should be a unit test
func (s *DockerSuite) TestRunNonExistingImage(c *testing.T) {
	icmd.RunCommand(dockerBinary, "run", "foo").Assert(c, icmd.Expected{
		ExitCode: 125,
		Err:      "Unable to find image",
	})
}

// TestDockerFails checks that 'docker run -foo busybox' exits with 125 to signal docker run failed
// FIXME(vdemeester) should be a unit test
func (s *DockerSuite) TestDockerFails(c *testing.T) {
	icmd.RunCommand(dockerBinary, "run", "-foo", "busybox").Assert(c, icmd.Expected{
		ExitCode: 125,
		Error:    "exit status 125",
	})
}

// TestRunInvalidReference invokes docker run with a bad reference.
func (s *DockerSuite) TestRunInvalidReference(c *testing.T) {
	out, exit, _ := dockerCmdWithError("run", "busybox@foo")
	if exit == 0 {
		c.Fatalf("expected non-zero exist code; received %d", exit)
	}

	if !strings.Contains(out, "invalid reference format") {
		c.Fatalf(`Expected "invalid reference format" in output; got: %s`, out)
	}
}

// Test fix for issue #17854
func (s *DockerSuite) TestRunInitLayerPathOwnership(c *testing.T) {
	// Not applicable on Windows as it does not support Linux uid/gid ownership
	testRequires(c, DaemonIsLinux)
	name := "testetcfileownership"
	buildImageSuccessfully(c, name, build.WithDockerfile(`FROM busybox
		RUN echo 'dockerio:x:1001:1001::/bin:/bin/false' >> /etc/passwd
		RUN echo 'dockerio:x:1001:' >> /etc/group
		RUN chown dockerio:dockerio /etc`))

	// Test that dockerio ownership of /etc is retained at runtime
	out, _ := dockerCmd(c, "run", "--rm", name, "stat", "-c", "%U:%G", "/etc")
	out = strings.TrimSpace(out)
	if out != "dockerio:dockerio" {
		c.Fatalf("Wrong /etc ownership: expected dockerio:dockerio, got %q", out)
	}
}

func (s *DockerSuite) TestRunWithOomScoreAdj(c *testing.T) {
	testRequires(c, DaemonIsLinux)

	expected := "642"
	out, _ := dockerCmd(c, "run", "--oom-score-adj", expected, "busybox", "cat", "/proc/self/oom_score_adj")
	oomScoreAdj := strings.TrimSpace(out)
	if oomScoreAdj != "642" {
		c.Fatalf("Expected oom_score_adj set to %q, got %q instead", expected, oomScoreAdj)
	}
}

func (s *DockerSuite) TestRunWithOomScoreAdjInvalidRange(c *testing.T) {
	testRequires(c, DaemonIsLinux)

	out, _, err := dockerCmdWithError("run", "--oom-score-adj", "1001", "busybox", "true")
	assert.ErrorContains(c, err, "")
	expected := "Invalid value 1001, range for oom score adj is [-1000, 1000]."
	if !strings.Contains(out, expected) {
		c.Fatalf("Expected output to contain %q, got %q instead", expected, out)
	}
	out, _, err = dockerCmdWithError("run", "--oom-score-adj", "-1001", "busybox", "true")
	assert.ErrorContains(c, err, "")
	expected = "Invalid value -1001, range for oom score adj is [-1000, 1000]."
	if !strings.Contains(out, expected) {
		c.Fatalf("Expected output to contain %q, got %q instead", expected, out)
	}
}

func (s *DockerSuite) TestRunNamedVolumesMountedAsShared(c *testing.T) {
	testRequires(c, DaemonIsLinux, NotUserNamespace)
	out, exitCode, _ := dockerCmdWithError("run", "-v", "foo:/test:shared", "busybox", "touch", "/test/somefile")
	assert.Assert(c, exitCode != 0)
	assert.Assert(c, strings.Contains(out, "invalid mount config"))
}

func (s *DockerSuite) TestRunNamedVolumeCopyImageData(c *testing.T) {
	testRequires(c, DaemonIsLinux)

	testImg := "testvolumecopy"
	buildImageSuccessfully(c, testImg, build.WithDockerfile(`
	FROM busybox
	RUN mkdir -p /foo && echo hello > /foo/hello
	`))

	dockerCmd(c, "run", "-v", "foo:/foo", testImg)
	out, _ := dockerCmd(c, "run", "-v", "foo:/foo", "busybox", "cat", "/foo/hello")
	assert.Equal(c, strings.TrimSpace(out), "hello")
}

func (s *DockerSuite) TestRunNamedVolumeNotRemoved(c *testing.T) {
	prefix, _ := getPrefixAndSlashFromDaemonPlatform()

	dockerCmd(c, "volume", "create", "test")

	dockerCmd(c, "run", "--rm", "-v", "test:"+prefix+"/foo", "-v", prefix+"/bar", "busybox", "true")
	dockerCmd(c, "volume", "inspect", "test")
	out, _ := dockerCmd(c, "volume", "ls", "-q")
	assert.Assert(c, strings.Contains(out, "test"))

	dockerCmd(c, "run", "--name=test", "-v", "test:"+prefix+"/foo", "-v", prefix+"/bar", "busybox", "true")
	dockerCmd(c, "rm", "-fv", "test")
	dockerCmd(c, "volume", "inspect", "test")
	out, _ = dockerCmd(c, "volume", "ls", "-q")
	assert.Assert(c, strings.Contains(out, "test"))
}

func (s *DockerSuite) TestRunNamedVolumesFromNotRemoved(c *testing.T) {
	prefix, _ := getPrefixAndSlashFromDaemonPlatform()

	dockerCmd(c, "volume", "create", "test")
	cid, _ := dockerCmd(c, "run", "-d", "--name=parent", "-v", "test:"+prefix+"/foo", "-v", prefix+"/bar", "busybox", "true")
	dockerCmd(c, "run", "--name=child", "--volumes-from=parent", "busybox", "true")

	cli, err := client.NewClientWithOpts(client.FromEnv)
	assert.NilError(c, err)
	defer cli.Close()

	container, err := cli.ContainerInspect(context.Background(), strings.TrimSpace(cid))
	assert.NilError(c, err)
	var vname string
	for _, v := range container.Mounts {
		if v.Name != "test" {
			vname = v.Name
		}
	}
	assert.Assert(c, vname != "")

	// Remove the parent so there are not other references to the volumes
	dockerCmd(c, "rm", "-f", "parent")
	// now remove the child and ensure the named volume (and only the named volume) still exists
	dockerCmd(c, "rm", "-fv", "child")
	dockerCmd(c, "volume", "inspect", "test")
	out, _ := dockerCmd(c, "volume", "ls", "-q")
	assert.Assert(c, strings.Contains(out, "test"))
	assert.Assert(c, !strings.Contains(strings.TrimSpace(out), vname))
}

func (s *DockerSuite) TestRunAttachFailedNoLeak(c *testing.T) {
	nroutines, err := getGoroutineNumber()
	assert.NilError(c, err)

	runSleepingContainer(c, "--name=test", "-p", "8000:8000")

	// Wait until container is fully up and running
	assert.Assert(c, waitRun("test") == nil)

	out, _, err := dockerCmdWithError("run", "--name=fail", "-p", "8000:8000", "busybox", "true")
	// We will need the following `inspect` to diagnose the issue if test fails (#21247)
	out1, err1 := dockerCmd(c, "inspect", "--format", "{{json .State}}", "test")
	out2, err2 := dockerCmd(c, "inspect", "--format", "{{json .State}}", "fail")
	assert.Assert(c, err != nil, "Command should have failed but succeeded with: %s\nContainer 'test' [%+v]: %s\nContainer 'fail' [%+v]: %s", out, err1, out1, err2, out2)
	// check for windows error as well
	// TODO Windows Post TP5. Fix the error message string
	outLowerCase := strings.ToLower(out)
	assert.Assert(c, strings.Contains(outLowerCase, "port is already allocated") ||
		strings.Contains(outLowerCase, "were not connected because a duplicate name exists") ||
		strings.Contains(outLowerCase, "the specified port already exists") ||
		strings.Contains(outLowerCase, "hns failed with error : failed to create endpoint") ||
		strings.Contains(outLowerCase, "hns failed with error : the object already exists"), fmt.Sprintf("Output: %s", out))
	dockerCmd(c, "rm", "-f", "test")

	// NGoroutines is not updated right away, so we need to wait before failing
	assert.Assert(c, waitForGoroutines(nroutines) == nil)
}

// Test for one character directory name case (#20122)
func (s *DockerSuite) TestRunVolumeWithOneCharacter(c *testing.T) {
	testRequires(c, DaemonIsLinux)

	out, _ := dockerCmd(c, "run", "-v", "/tmp/q:/foo", "busybox", "sh", "-c", "find /foo")
	assert.Equal(c, strings.TrimSpace(out), "/foo")
}

func (s *DockerSuite) TestRunVolumeCopyFlag(c *testing.T) {
	testRequires(c, DaemonIsLinux) // Windows does not support copying data from image to the volume
	buildImageSuccessfully(c, "volumecopy", build.WithDockerfile(`FROM busybox
		RUN mkdir /foo && echo hello > /foo/bar
		CMD cat /foo/bar`))
	dockerCmd(c, "volume", "create", "test")

	// test with the nocopy flag
	out, _, err := dockerCmdWithError("run", "-v", "test:/foo:nocopy", "volumecopy")
	assert.ErrorContains(c, err, "", out)
	// test default behavior which is to copy for non-binds
	out, _ = dockerCmd(c, "run", "-v", "test:/foo", "volumecopy")
	assert.Equal(c, strings.TrimSpace(out), "hello")
	// error out when the volume is already populated
	out, _, err = dockerCmdWithError("run", "-v", "test:/foo:copy", "volumecopy")
	assert.ErrorContains(c, err, "", out)
	// do not error out when copy isn't explicitly set even though it's already populated
	out, _ = dockerCmd(c, "run", "-v", "test:/foo", "volumecopy")
	assert.Equal(c, strings.TrimSpace(out), "hello")

	// do not allow copy modes on volumes-from
	dockerCmd(c, "run", "--name=test", "-v", "/foo", "busybox", "true")
	out, _, err = dockerCmdWithError("run", "--volumes-from=test:copy", "busybox", "true")
	assert.ErrorContains(c, err, "", out)
	out, _, err = dockerCmdWithError("run", "--volumes-from=test:nocopy", "busybox", "true")
	assert.ErrorContains(c, err, "", out)

	// do not allow copy modes on binds
	out, _, err = dockerCmdWithError("run", "-v", "/foo:/bar:copy", "busybox", "true")
	assert.ErrorContains(c, err, "", out)
	out, _, err = dockerCmdWithError("run", "-v", "/foo:/bar:nocopy", "busybox", "true")
	assert.ErrorContains(c, err, "", out)
}

// Test case for #21976
func (s *DockerSuite) TestRunDNSInHostMode(c *testing.T) {
	testRequires(c, DaemonIsLinux, NotUserNamespace)

	expectedOutput := "nameserver 127.0.0.1"
	expectedWarning := "Localhost DNS setting"
	cli.DockerCmd(c, "run", "--dns=127.0.0.1", "--net=host", "busybox", "cat", "/etc/resolv.conf").Assert(c, icmd.Expected{
		Out: expectedOutput,
		Err: expectedWarning,
	})

	expectedOutput = "nameserver 1.2.3.4"
	cli.DockerCmd(c, "run", "--dns=1.2.3.4", "--net=host", "busybox", "cat", "/etc/resolv.conf").Assert(c, icmd.Expected{
		Out: expectedOutput,
	})

	expectedOutput = "search example.com"
	cli.DockerCmd(c, "run", "--dns-search=example.com", "--net=host", "busybox", "cat", "/etc/resolv.conf").Assert(c, icmd.Expected{
		Out: expectedOutput,
	})

	expectedOutput = "options timeout:3"
	cli.DockerCmd(c, "run", "--dns-opt=timeout:3", "--net=host", "busybox", "cat", "/etc/resolv.conf").Assert(c, icmd.Expected{
		Out: expectedOutput,
	})

	expectedOutput1 := "nameserver 1.2.3.4"
	expectedOutput2 := "search example.com"
	expectedOutput3 := "options timeout:3"
	out := cli.DockerCmd(c, "run", "--dns=1.2.3.4", "--dns-search=example.com", "--dns-opt=timeout:3", "--net=host", "busybox", "cat", "/etc/resolv.conf").Combined()
	assert.Assert(c, strings.Contains(out, expectedOutput1), "Expected '%s', but got %q", expectedOutput1, out)
	assert.Assert(c, strings.Contains(out, expectedOutput2), "Expected '%s', but got %q", expectedOutput2, out)
	assert.Assert(c, strings.Contains(out, expectedOutput3), "Expected '%s', but got %q", expectedOutput3, out)
}

// Test case for #21976
func (s *DockerSuite) TestRunAddHostInHostMode(c *testing.T) {
	testRequires(c, DaemonIsLinux, NotUserNamespace)

	expectedOutput := "1.2.3.4\textra"
	out, _ := dockerCmd(c, "run", "--add-host=extra:1.2.3.4", "--net=host", "busybox", "cat", "/etc/hosts")
	assert.Assert(c, strings.Contains(out, expectedOutput), "Expected '%s', but got %q", expectedOutput, out)
}

func (s *DockerSuite) TestRunRmAndWait(c *testing.T) {
	dockerCmd(c, "run", "--name=test", "--rm", "-d", "busybox", "sh", "-c", "sleep 3;exit 2")

	out, code, err := dockerCmdWithError("wait", "test")
	assert.Assert(c, err == nil, "out: %s; exit code: %d", out, code)
	assert.Equal(c, out, "2\n", "exit code: %d", code)
	assert.Equal(c, code, 0)
}

// Test that auto-remove is performed by the daemon (API 1.25 and above)
func (s *DockerSuite) TestRunRm(c *testing.T) {
	name := "miss-me-when-im-gone"
	cli.DockerCmd(c, "run", "--name="+name, "--rm", "busybox")

	cli.Docker(cli.Inspect(name), cli.Format(".name")).Assert(c, icmd.Expected{
		ExitCode: 1,
		Err:      "No such object: " + name,
	})
}

// Test that auto-remove is performed by the client on API versions that do not support daemon-side api-remove (API < 1.25)
func (s *DockerSuite) TestRunRmPre125Api(c *testing.T) {
	name := "miss-me-when-im-gone"
	envs := appendBaseEnv(os.Getenv("DOCKER_TLS_VERIFY") != "", "DOCKER_API_VERSION=1.24")
	cli.Docker(cli.Args("run", "--name="+name, "--rm", "busybox"), cli.WithEnvironmentVariables(envs...)).Assert(c, icmd.Success)

	cli.Docker(cli.Inspect(name), cli.Format(".name")).Assert(c, icmd.Expected{
		ExitCode: 1,
		Err:      "No such object: " + name,
	})
}

// Test case for #23498
func (s *DockerSuite) TestRunUnsetEntrypoint(c *testing.T) {
	testRequires(c, DaemonIsLinux)
	name := "test-entrypoint"
	dockerfile := `FROM busybox
ADD entrypoint.sh /entrypoint.sh
RUN chmod 755 /entrypoint.sh
ENTRYPOINT ["/entrypoint.sh"]
CMD echo foobar`

	ctx := fakecontext.New(c, "",
		fakecontext.WithDockerfile(dockerfile),
		fakecontext.WithFiles(map[string]string{
			"entrypoint.sh": `#!/bin/sh
echo "I am an entrypoint"
exec "$@"`,
		}))
	defer ctx.Close()

	cli.BuildCmd(c, name, build.WithExternalBuildContext(ctx))

	out := cli.DockerCmd(c, "run", "--entrypoint=", "-t", name, "echo", "foo").Combined()
	assert.Equal(c, strings.TrimSpace(out), "foo")

	// CMD will be reset as well (the same as setting a custom entrypoint)
	cli.Docker(cli.Args("run", "--entrypoint=", "-t", name)).Assert(c, icmd.Expected{
		ExitCode: 125,
		Err:      "No command specified",
	})
}

func (s *DockerDaemonSuite) TestRunWithUlimitAndDaemonDefault(c *testing.T) {
	s.d.StartWithBusybox(c, "--debug", "--default-ulimit=nofile=65535")

	name := "test-A"
	_, err := s.d.Cmd("run", "--name", name, "-d", "busybox", "top")
	assert.NilError(c, err)
	assert.NilError(c, s.d.WaitRun(name))

	out, err := s.d.Cmd("inspect", "--format", "{{.HostConfig.Ulimits}}", name)
	assert.NilError(c, err)
	assert.Assert(c, strings.Contains(out, "[nofile=65535:65535]"))
	name = "test-B"
	_, err = s.d.Cmd("run", "--name", name, "--ulimit=nofile=42", "-d", "busybox", "top")
	assert.NilError(c, err)
	assert.NilError(c, s.d.WaitRun(name))

	out, err = s.d.Cmd("inspect", "--format", "{{.HostConfig.Ulimits}}", name)
	assert.NilError(c, err)
	assert.Assert(c, strings.Contains(out, "[nofile=42:42]"))
}

func (s *DockerSuite) TestRunStoppedLoggingDriverNoLeak(c *testing.T) {
	nroutines, err := getGoroutineNumber()
	assert.NilError(c, err)

	out, _, err := dockerCmdWithError("run", "--name=fail", "--log-driver=splunk", "busybox", "true")
	assert.ErrorContains(c, err, "")
	assert.Assert(c, strings.Contains(out, "failed to initialize logging driver"), "error should be about logging driver, got output %s", out)
	// NGoroutines is not updated right away, so we need to wait before failing
	assert.Assert(c, waitForGoroutines(nroutines) == nil)
}

// Handles error conditions for --credentialspec. Validating E2E success cases
// requires additional infrastructure (AD for example) on CI servers.
func (s *DockerSuite) TestRunCredentialSpecFailures(c *testing.T) {
	testRequires(c, DaemonIsWindows)

	attempts := []struct{ value, expectedError string }{
		{"rubbish", "invalid credential spec security option - value must be prefixed by 'file://', 'registry://', or 'raw://' followed by a non-empty value"},
		{"rubbish://", "invalid credential spec security option - value must be prefixed by 'file://', 'registry://', or 'raw://' followed by a non-empty value"},
		{"file://", "invalid credential spec security option - value must be prefixed by 'file://', 'registry://', or 'raw://' followed by a non-empty value"},
		{"registry://", "invalid credential spec security option - value must be prefixed by 'file://', 'registry://', or 'raw://' followed by a non-empty value"},
		{`file://c:\blah.txt`, "path cannot be absolute"},
		{`file://doesnotexist.txt`, "The system cannot find the file specified"},
	}
	for _, attempt := range attempts {
		_, _, err := dockerCmdWithError("run", "--security-opt=credentialspec="+attempt.value, "busybox", "true")
		assert.Assert(c, err != nil, "%s expected non-nil err", attempt.value)
		assert.Assert(c, strings.Contains(err.Error(), attempt.expectedError), "%s expected %s got %s", attempt.value, attempt.expectedError, err)
	}
}

// Windows specific test to validate credential specs with a well-formed spec.
func (s *DockerSuite) TestRunCredentialSpecWellFormed(c *testing.T) {
	testRequires(c, DaemonIsWindows, testEnv.IsLocalDaemon)

	validCredSpecs := readFile(`fixtures\credentialspecs\valid.json`, c)
	writeFile(filepath.Join(testEnv.DaemonInfo.DockerRootDir, `credentialspecs\valid.json`), validCredSpecs, c)

	for _, value := range []string{"file://valid.json", "raw://" + validCredSpecs} {
		// `nltest /PARENTDOMAIN` simply reads the local config, and does not require having an AD
		// controller handy
		out, _ := dockerCmd(c, "run", "--rm", "--security-opt=credentialspec="+value, minimalBaseImage(), "nltest", "/PARENTDOMAIN")

		assert.Assert(c, strings.Contains(out, "hyperv.local."))
		assert.Assert(c, strings.Contains(out, "The command completed successfully"))
	}
}

func (s *DockerSuite) TestRunDuplicateMount(c *testing.T) {
	testRequires(c, testEnv.IsLocalDaemon, DaemonIsLinux, NotUserNamespace)

	tmpFile, err := ioutil.TempFile("", "touch-me")
	assert.NilError(c, err)
	defer tmpFile.Close()

	data := "touch-me-foo-bar\n"
	if _, err := tmpFile.Write([]byte(data)); err != nil {
		c.Fatal(err)
	}

	name := "test"
	out, _ := dockerCmd(c, "run", "--name", name, "-v", "/tmp:/tmp", "-v", "/tmp:/tmp", "busybox", "sh", "-c", "cat "+tmpFile.Name()+" && ls /")
	assert.Assert(c, !strings.Contains(out, "tmp:"))
	assert.Assert(c, strings.Contains(out, data))
	out = inspectFieldJSON(c, name, "Config.Volumes")
	assert.Assert(c, strings.Contains(out, "null"))
}

func (s *DockerSuite) TestRunWindowsWithCPUCount(c *testing.T) {
	testRequires(c, DaemonIsWindows)

	out, _ := dockerCmd(c, "run", "--cpu-count=1", "--name", "test", "busybox", "echo", "testing")
	assert.Equal(c, strings.TrimSpace(out), "testing")

	out = inspectField(c, "test", "HostConfig.CPUCount")
	assert.Equal(c, out, "1")
}

func (s *DockerSuite) TestRunWindowsWithCPUShares(c *testing.T) {
	testRequires(c, DaemonIsWindows)

	out, _ := dockerCmd(c, "run", "--cpu-shares=1000", "--name", "test", "busybox", "echo", "testing")
	assert.Equal(c, strings.TrimSpace(out), "testing")

	out = inspectField(c, "test", "HostConfig.CPUShares")
	assert.Equal(c, out, "1000")
}

func (s *DockerSuite) TestRunWindowsWithCPUPercent(c *testing.T) {
	testRequires(c, DaemonIsWindows)

	out, _ := dockerCmd(c, "run", "--cpu-percent=80", "--name", "test", "busybox", "echo", "testing")
	assert.Equal(c, strings.TrimSpace(out), "testing")

	out = inspectField(c, "test", "HostConfig.CPUPercent")
	assert.Equal(c, out, "80")
}

func (s *DockerSuite) TestRunProcessIsolationWithCPUCountCPUSharesAndCPUPercent(c *testing.T) {
	testRequires(c, IsolationIsProcess)

	out, _ := dockerCmd(c, "run", "--cpu-count=1", "--cpu-shares=1000", "--cpu-percent=80", "--name", "test", "busybox", "echo", "testing")
	assert.Assert(c, strings.Contains(strings.TrimSpace(out), "WARNING: Conflicting options: CPU count takes priority over CPU shares on Windows Server Containers. CPU shares discarded"))
	assert.Assert(c, strings.Contains(strings.TrimSpace(out), "WARNING: Conflicting options: CPU count takes priority over CPU percent on Windows Server Containers. CPU percent discarded"))
	assert.Assert(c, strings.Contains(strings.TrimSpace(out), "testing"))
	out = inspectField(c, "test", "HostConfig.CPUCount")
	assert.Equal(c, out, "1")

	out = inspectField(c, "test", "HostConfig.CPUShares")
	assert.Equal(c, out, "0")

	out = inspectField(c, "test", "HostConfig.CPUPercent")
	assert.Equal(c, out, "0")
}

func (s *DockerSuite) TestRunHypervIsolationWithCPUCountCPUSharesAndCPUPercent(c *testing.T) {
	testRequires(c, IsolationIsHyperv)

	out, _ := dockerCmd(c, "run", "--cpu-count=1", "--cpu-shares=1000", "--cpu-percent=80", "--name", "test", "busybox", "echo", "testing")
	assert.Assert(c, strings.Contains(strings.TrimSpace(out), "testing"))
	out = inspectField(c, "test", "HostConfig.CPUCount")
	assert.Equal(c, out, "1")

	out = inspectField(c, "test", "HostConfig.CPUShares")
	assert.Equal(c, out, "1000")

	out = inspectField(c, "test", "HostConfig.CPUPercent")
	assert.Equal(c, out, "80")
}

// Test for #25099
func (s *DockerSuite) TestRunEmptyEnv(c *testing.T) {
	testRequires(c, DaemonIsLinux)

	expectedOutput := "invalid environment variable:"

	out, _, err := dockerCmdWithError("run", "-e", "", "busybox", "true")
	assert.ErrorContains(c, err, "")
	assert.Assert(c, strings.Contains(out, expectedOutput))

	out, _, err = dockerCmdWithError("run", "-e", "=", "busybox", "true")
	assert.ErrorContains(c, err, "")
	assert.Assert(c, strings.Contains(out, expectedOutput))

	out, _, err = dockerCmdWithError("run", "-e", "=foo", "busybox", "true")
	assert.ErrorContains(c, err, "")
	assert.Assert(c, strings.Contains(out, expectedOutput))
}

// #28658
func (s *DockerSuite) TestSlowStdinClosing(c *testing.T) {
	name := "testslowstdinclosing"
	repeat := 3 // regression happened 50% of the time
	for i := 0; i < repeat; i++ {
		cmd := icmd.Cmd{
			Command: []string{dockerBinary, "run", "--rm", "--name", name, "-i", "busybox", "cat"},
			Stdin:   &delayedReader{},
		}
		done := make(chan error, 1)
		go func() {
			err := icmd.RunCmd(cmd).Error
			done <- err
		}()

		select {
		case <-time.After(30 * time.Second):
			c.Fatal("running container timed out") // cleanup in teardown
		case err := <-done:
			assert.NilError(c, err)
		}
	}
}

type delayedReader struct{}

func (s *delayedReader) Read([]byte) (int, error) {
	time.Sleep(500 * time.Millisecond)
	return 0, io.EOF
}

// #28823 (originally #28639)
func (s *DockerSuite) TestRunMountReadOnlyDevShm(c *testing.T) {
	testRequires(c, testEnv.IsLocalDaemon, DaemonIsLinux, NotUserNamespace)
	emptyDir, err := ioutil.TempDir("", "test-read-only-dev-shm")
	assert.NilError(c, err)
	defer os.RemoveAll(emptyDir)
	out, _, err := dockerCmdWithError("run", "--rm", "--read-only",
		"-v", fmt.Sprintf("%s:/dev/shm:ro", emptyDir),
		"busybox", "touch", "/dev/shm/foo")
	assert.ErrorContains(c, err, "", out)
	assert.Assert(c, strings.Contains(out, "Read-only file system"))
}

func (s *DockerSuite) TestRunMount(c *testing.T) {
	testRequires(c, DaemonIsLinux, testEnv.IsLocalDaemon, NotUserNamespace)

	// mnt1, mnt2, and testCatFooBar are commonly used in multiple test cases
	tmpDir, err := ioutil.TempDir("", "mount")
	if err != nil {
		c.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)
	mnt1, mnt2 := path.Join(tmpDir, "mnt1"), path.Join(tmpDir, "mnt2")
	if err := os.Mkdir(mnt1, 0755); err != nil {
		c.Fatal(err)
	}
	if err := os.Mkdir(mnt2, 0755); err != nil {
		c.Fatal(err)
	}
	if err := ioutil.WriteFile(path.Join(mnt1, "test1"), []byte("test1"), 0644); err != nil {
		c.Fatal(err)
	}
	if err := ioutil.WriteFile(path.Join(mnt2, "test2"), []byte("test2"), 0644); err != nil {
		c.Fatal(err)
	}
	testCatFooBar := func(cName string) error {
		out, _ := dockerCmd(c, "exec", cName, "cat", "/foo/test1")
		if out != "test1" {
			return fmt.Errorf("%s not mounted on /foo", mnt1)
		}
		out, _ = dockerCmd(c, "exec", cName, "cat", "/bar/test2")
		if out != "test2" {
			return fmt.Errorf("%s not mounted on /bar", mnt2)
		}
		return nil
	}

	type testCase struct {
		equivalents [][]string
		valid       bool
		// fn should be nil if valid==false
		fn func(cName string) error
	}
	cases := []testCase{
		{
			equivalents: [][]string{
				{
					"--mount", fmt.Sprintf("type=bind,src=%s,dst=/foo", mnt1),
					"--mount", fmt.Sprintf("type=bind,src=%s,dst=/bar", mnt2),
				},
				{
					"--mount", fmt.Sprintf("type=bind,src=%s,dst=/foo", mnt1),
					"--mount", fmt.Sprintf("type=bind,src=%s,target=/bar", mnt2),
				},
				{
					"--volume", mnt1 + ":/foo",
					"--mount", fmt.Sprintf("type=bind,src=%s,target=/bar", mnt2),
				},
			},
			valid: true,
			fn:    testCatFooBar,
		},
		{
			equivalents: [][]string{
				{
					"--mount", fmt.Sprintf("type=volume,src=%s,dst=/foo", mnt1),
					"--mount", fmt.Sprintf("type=volume,src=%s,dst=/bar", mnt2),
				},
				{
					"--mount", fmt.Sprintf("type=volume,src=%s,dst=/foo", mnt1),
					"--mount", fmt.Sprintf("type=volume,src=%s,target=/bar", mnt2),
				},
			},
			valid: false,
		},
		{
			equivalents: [][]string{
				{
					"--mount", fmt.Sprintf("type=bind,src=%s,dst=/foo", mnt1),
					"--mount", fmt.Sprintf("type=volume,src=%s,dst=/bar", mnt2),
				},
				{
					"--volume", mnt1 + ":/foo",
					"--mount", fmt.Sprintf("type=volume,src=%s,target=/bar", mnt2),
				},
			},
			valid: false,
			fn:    testCatFooBar,
		},
		{
			equivalents: [][]string{
				{
					"--read-only",
					"--mount", "type=volume,dst=/bar",
				},
			},
			valid: true,
			fn: func(cName string) error {
				_, _, err := dockerCmdWithError("exec", cName, "touch", "/bar/icanwritehere")
				return err
			},
		},
		{
			equivalents: [][]string{
				{
					"--read-only",
					"--mount", fmt.Sprintf("type=bind,src=%s,dst=/foo", mnt1),
					"--mount", "type=volume,dst=/bar",
				},
				{
					"--read-only",
					"--volume", fmt.Sprintf("%s:/foo", mnt1),
					"--mount", "type=volume,dst=/bar",
				},
			},
			valid: true,
			fn: func(cName string) error {
				out, _ := dockerCmd(c, "exec", cName, "cat", "/foo/test1")
				if out != "test1" {
					return fmt.Errorf("%s not mounted on /foo", mnt1)
				}
				_, _, err := dockerCmdWithError("exec", cName, "touch", "/bar/icanwritehere")
				return err
			},
		},
		{
			equivalents: [][]string{
				{
					"--mount", fmt.Sprintf("type=bind,src=%s,dst=/foo", mnt1),
					"--mount", fmt.Sprintf("type=bind,src=%s,dst=/foo", mnt2),
				},
				{
					"--mount", fmt.Sprintf("type=bind,src=%s,dst=/foo", mnt1),
					"--mount", fmt.Sprintf("type=bind,src=%s,target=/foo", mnt2),
				},
				{
					"--volume", fmt.Sprintf("%s:/foo", mnt1),
					"--mount", fmt.Sprintf("type=bind,src=%s,target=/foo", mnt2),
				},
			},
			valid: false,
		},
		{
			equivalents: [][]string{
				{
					"--volume", fmt.Sprintf("%s:/foo", mnt1),
					"--mount", fmt.Sprintf("type=volume,src=%s,target=/foo", mnt2),
				},
			},
			valid: false,
		},
		{
			equivalents: [][]string{
				{
					"--mount", "type=volume,target=/foo",
					"--mount", "type=volume,target=/foo",
				},
			},
			valid: false,
		},
	}

	for i, testCase := range cases {
		for j, opts := range testCase.equivalents {
			cName := fmt.Sprintf("mount-%d-%d", i, j)
			_, _, err := dockerCmdWithError(append([]string{"run", "-i", "-d", "--name", cName},
				append(opts, []string{"busybox", "top"}...)...)...)
			if testCase.valid {
				assert.Assert(c, err == nil, "got error while creating a container with %v (%s)", opts, cName)
				assert.Assert(c, testCase.fn(cName) == nil, "got error while executing test for %v (%s)", opts, cName)
				dockerCmd(c, "rm", "-f", cName)
			} else {
				assert.Assert(c, err != nil, "got nil while creating a container with %v (%s)", opts, cName)
			}
		}
	}
}

// Test that passing a FQDN as hostname properly sets hostname, and
// /etc/hostname. Test case for 29100
func (s *DockerSuite) TestRunHostnameFQDN(c *testing.T) {
	testRequires(c, DaemonIsLinux)

	expectedOutput := "foobar.example.com\nfoobar.example.com\nfoobar\nexample.com\nfoobar.example.com"
	out, _ := dockerCmd(c, "run", "--hostname=foobar.example.com", "busybox", "sh", "-c", `cat /etc/hostname && hostname && hostname -s && hostname -d && hostname -f`)
	assert.Equal(c, strings.TrimSpace(out), expectedOutput)

	out, _ = dockerCmd(c, "run", "--hostname=foobar.example.com", "busybox", "sh", "-c", `cat /etc/hosts`)
	expectedOutput = "foobar.example.com foobar"
	assert.Assert(c, strings.Contains(strings.TrimSpace(out), expectedOutput))
}

// Test case for 29129
func (s *DockerSuite) TestRunHostnameInHostMode(c *testing.T) {
	testRequires(c, DaemonIsLinux, NotUserNamespace)

	expectedOutput := "foobar\nfoobar"
	out, _ := dockerCmd(c, "run", "--net=host", "--hostname=foobar", "busybox", "sh", "-c", `echo $HOSTNAME && hostname`)
	assert.Equal(c, strings.TrimSpace(out), expectedOutput)
}

func (s *DockerSuite) TestRunAddDeviceCgroupRule(c *testing.T) {
	testRequires(c, DaemonIsLinux)

	deviceRule := "c 7:128 rwm"

	out, _ := dockerCmd(c, "run", "--rm", "busybox", "cat", "/sys/fs/cgroup/devices/devices.list")
	if strings.Contains(out, deviceRule) {
		c.Fatalf("%s shouldn't been in the device.list", deviceRule)
	}

	out, _ = dockerCmd(c, "run", "--rm", fmt.Sprintf("--device-cgroup-rule=%s", deviceRule), "busybox", "grep", deviceRule, "/sys/fs/cgroup/devices/devices.list")
	assert.Equal(c, strings.TrimSpace(out), deviceRule)
}

// Verifies that running as local system is operating correctly on Windows
func (s *DockerSuite) TestWindowsRunAsSystem(c *testing.T) {
	testRequires(c, DaemonIsWindowsAtLeastBuild(osversion.RS3))
	out, _ := dockerCmd(c, "run", "--net=none", `--user=nt authority\system`, "--hostname=XYZZY", minimalBaseImage(), "cmd", "/c", `@echo %USERNAME%`)
	assert.Equal(c, strings.TrimSpace(out), "XYZZY$")
}
