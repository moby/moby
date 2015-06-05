package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/nat"
	"github.com/docker/libnetwork/resolvconf"
	"github.com/go-check/check"
)

// "test123" should be printed by docker run
func (s *DockerSuite) TestRunEchoStdout(c *check.C) {
	runCmd := exec.Command(dockerBinary, "run", "busybox", "echo", "test123")
	out, _, _, err := runCommandWithStdoutStderr(runCmd)
	if err != nil {
		c.Fatalf("failed to run container: %v, output: %q", err, out)
	}

	if out != "test123\n" {
		c.Fatalf("container should've printed 'test123'")
	}
}

// "test" should be printed
func (s *DockerSuite) TestRunEchoStdoutWithMemoryLimit(c *check.C) {
	runCmd := exec.Command(dockerBinary, "run", "-m", "16m", "busybox", "echo", "test")
	out, _, _, err := runCommandWithStdoutStderr(runCmd)
	if err != nil {
		c.Fatalf("failed to run container: %v, output: %q", err, out)
	}

	out = strings.Trim(out, "\r\n")

	if expected := "test"; out != expected {
		c.Fatalf("container should've printed %q but printed %q", expected, out)
	}
}

// should run without memory swap
func (s *DockerSuite) TestRunWithoutMemoryswapLimit(c *check.C) {
	testRequires(c, NativeExecDriver)
	runCmd := exec.Command(dockerBinary, "run", "-m", "16m", "--memory-swap", "-1", "busybox", "true")
	out, _, err := runCommandWithOutput(runCmd)
	if err != nil {
		c.Fatalf("failed to run container, output: %q", out)
	}
}

// "test" should be printed
func (s *DockerSuite) TestRunEchoStdoutWitCPULimit(c *check.C) {
	runCmd := exec.Command(dockerBinary, "run", "-c", "1000", "busybox", "echo", "test")
	out, _, _, err := runCommandWithStdoutStderr(runCmd)
	if err != nil {
		c.Fatalf("failed to run container: %v, output: %q", err, out)
	}

	if out != "test\n" {
		c.Errorf("container should've printed 'test'")
	}
}

// "test" should be printed
func (s *DockerSuite) TestRunEchoStdoutWithCPUAndMemoryLimit(c *check.C) {
	runCmd := exec.Command(dockerBinary, "run", "-c", "1000", "-m", "16m", "busybox", "echo", "test")
	out, _, _, err := runCommandWithStdoutStderr(runCmd)
	if err != nil {
		c.Fatalf("failed to run container: %v, output: %q", err, out)
	}

	if out != "test\n" {
		c.Errorf("container should've printed 'test', got %q instead", out)
	}
}

// "test" should be printed
func (s *DockerSuite) TestRunEchoNamedContainer(c *check.C) {
	runCmd := exec.Command(dockerBinary, "run", "--name", "testfoonamedcontainer", "busybox", "echo", "test")
	out, _, _, err := runCommandWithStdoutStderr(runCmd)
	if err != nil {
		c.Fatalf("failed to run container: %v, output: %q", err, out)
	}

	if out != "test\n" {
		c.Errorf("container should've printed 'test'")
	}
}

// docker run should not leak file descriptors
func (s *DockerSuite) TestRunLeakyFileDescriptors(c *check.C) {
	runCmd := exec.Command(dockerBinary, "run", "busybox", "ls", "-C", "/proc/self/fd")
	out, _, _, err := runCommandWithStdoutStderr(runCmd)
	if err != nil {
		c.Fatalf("failed to run container: %v, output: %q", err, out)
	}

	// normally, we should only get 0, 1, and 2, but 3 gets created by "ls" when it does "opendir" on the "fd" directory
	if out != "0  1  2  3\n" {
		c.Errorf("container should've printed '0  1  2  3', not: %s", out)
	}
}

// it should be possible to lookup Google DNS
// this will fail when Internet access is unavailable
func (s *DockerSuite) TestRunLookupGoogleDns(c *check.C) {
	testRequires(c, Network)

	out, _, _, err := runCommandWithStdoutStderr(exec.Command(dockerBinary, "run", "busybox", "nslookup", "google.com"))
	if err != nil {
		c.Fatalf("failed to run container: %v, output: %q", err, out)
	}
}

// the exit code should be 0
// some versions of lxc might make this test fail
func (s *DockerSuite) TestRunExitCodeZero(c *check.C) {
	runCmd := exec.Command(dockerBinary, "run", "busybox", "true")
	if out, _, err := runCommandWithOutput(runCmd); err != nil {
		c.Errorf("container should've exited with exit code 0: %s, %v", out, err)
	}
}

// the exit code should be 1
// some versions of lxc might make this test fail
func (s *DockerSuite) TestRunExitCodeOne(c *check.C) {
	runCmd := exec.Command(dockerBinary, "run", "busybox", "false")
	exitCode, err := runCommand(runCmd)
	if err != nil && !strings.Contains("exit status 1", fmt.Sprintf("%s", err)) {
		c.Fatal(err)
	}
	if exitCode != 1 {
		c.Errorf("container should've exited with exit code 1")
	}
}

// it should be possible to pipe in data via stdin to a process running in a container
// some versions of lxc might make this test fail
func (s *DockerSuite) TestRunStdinPipe(c *check.C) {
	runCmd := exec.Command(dockerBinary, "run", "-i", "-a", "stdin", "busybox", "cat")
	runCmd.Stdin = strings.NewReader("blahblah")
	out, _, _, err := runCommandWithStdoutStderr(runCmd)
	if err != nil {
		c.Fatalf("failed to run container: %v, output: %q", err, out)
	}

	out = strings.TrimSpace(out)
	waitCmd := exec.Command(dockerBinary, "wait", out)
	if waitOut, _, err := runCommandWithOutput(waitCmd); err != nil {
		c.Fatalf("error thrown while waiting for container: %s, %v", waitOut, err)
	}

	logsCmd := exec.Command(dockerBinary, "logs", out)
	logsOut, _, err := runCommandWithOutput(logsCmd)
	if err != nil {
		c.Fatalf("error thrown while trying to get container logs: %s, %v", logsOut, err)
	}

	containerLogs := strings.TrimSpace(logsOut)

	if containerLogs != "blahblah" {
		c.Errorf("logs didn't print the container's logs %s", containerLogs)
	}

	rmCmd := exec.Command(dockerBinary, "rm", out)
	if out, _, err = runCommandWithOutput(rmCmd); err != nil {
		c.Fatalf("rm failed to remove container: %s, %v", out, err)
	}
}

// the container's ID should be printed when starting a container in detached mode
func (s *DockerSuite) TestRunDetachedContainerIDPrinting(c *check.C) {
	runCmd := exec.Command(dockerBinary, "run", "-d", "busybox", "true")
	out, _, _, err := runCommandWithStdoutStderr(runCmd)
	if err != nil {
		c.Fatalf("failed to run container: %v, output: %q", err, out)
	}

	out = strings.TrimSpace(out)
	waitCmd := exec.Command(dockerBinary, "wait", out)
	if waitOut, _, err := runCommandWithOutput(waitCmd); err != nil {
		c.Fatalf("error thrown while waiting for container: %s, %v", waitOut, err)
	}

	rmCmd := exec.Command(dockerBinary, "rm", out)
	rmOut, _, err := runCommandWithOutput(rmCmd)
	if err != nil {
		c.Fatalf("rm failed to remove container: %s, %v", rmOut, err)
	}

	rmOut = strings.TrimSpace(rmOut)
	if rmOut != out {
		c.Errorf("rm didn't print the container ID %s %s", out, rmOut)
	}
}

// the working directory should be set correctly
func (s *DockerSuite) TestRunWorkingDirectory(c *check.C) {
	runCmd := exec.Command(dockerBinary, "run", "-w", "/root", "busybox", "pwd")
	out, _, _, err := runCommandWithStdoutStderr(runCmd)
	if err != nil {
		c.Fatalf("failed to run container: %v, output: %q", err, out)
	}

	out = strings.TrimSpace(out)

	if out != "/root" {
		c.Errorf("-w failed to set working directory")
	}

	runCmd = exec.Command(dockerBinary, "run", "--workdir", "/root", "busybox", "pwd")
	out, _, _, err = runCommandWithStdoutStderr(runCmd)
	if err != nil {
		c.Fatal(out, err)
	}

	out = strings.TrimSpace(out)

	if out != "/root" {
		c.Errorf("--workdir failed to set working directory")
	}
}

// pinging Google's DNS resolver should fail when we disable the networking
func (s *DockerSuite) TestRunWithoutNetworking(c *check.C) {
	runCmd := exec.Command(dockerBinary, "run", "--net=none", "busybox", "ping", "-c", "1", "8.8.8.8")
	out, _, exitCode, err := runCommandWithStdoutStderr(runCmd)
	if err != nil && exitCode != 1 {
		c.Fatal(out, err)
	}
	if exitCode != 1 {
		c.Errorf("--net=none should've disabled the network; the container shouldn't have been able to ping 8.8.8.8")
	}

	runCmd = exec.Command(dockerBinary, "run", "-n=false", "busybox", "ping", "-c", "1", "8.8.8.8")
	out, _, exitCode, err = runCommandWithStdoutStderr(runCmd)
	if err != nil && exitCode != 1 {
		c.Fatal(out, err)
	}
	if exitCode != 1 {
		c.Errorf("-n=false should've disabled the network; the container shouldn't have been able to ping 8.8.8.8")
	}
}

//test --link use container name to link target
func (s *DockerSuite) TestRunLinksContainerWithContainerName(c *check.C) {
	cmd := exec.Command(dockerBinary, "run", "-i", "-t", "-d", "--name", "parent", "busybox")
	out, _, _, err := runCommandWithStdoutStderr(cmd)
	if err != nil {
		c.Fatalf("failed to run container: %v, output: %q", err, out)
	}
	ip, err := inspectField("parent", "NetworkSettings.IPAddress")
	c.Assert(err, check.IsNil)
	cmd = exec.Command(dockerBinary, "run", "--link", "parent:test", "busybox", "/bin/cat", "/etc/hosts")
	out, _, err = runCommandWithOutput(cmd)
	if err != nil {
		c.Fatalf("failed to run container: %v, output: %q", err, out)
	}
	if !strings.Contains(out, ip+"	test") {
		c.Fatalf("use a container name to link target failed")
	}
}

//test --link use container id to link target
func (s *DockerSuite) TestRunLinksContainerWithContainerId(c *check.C) {
	cmd := exec.Command(dockerBinary, "run", "-i", "-t", "-d", "busybox")
	cID, _, _, err := runCommandWithStdoutStderr(cmd)
	if err != nil {
		c.Fatalf("failed to run container: %v, output: %q", err, cID)
	}
	cID = strings.TrimSpace(cID)
	ip, err := inspectField(cID, "NetworkSettings.IPAddress")
	c.Assert(err, check.IsNil)
	cmd = exec.Command(dockerBinary, "run", "--link", cID+":test", "busybox", "/bin/cat", "/etc/hosts")
	out, _, err := runCommandWithOutput(cmd)
	if err != nil {
		c.Fatalf("failed to run container: %v, output: %q", err, out)
	}
	if !strings.Contains(out, ip+"	test") {
		c.Fatalf("use a container id to link target failed")
	}
}

func (s *DockerSuite) TestRunLinkToContainerNetMode(c *check.C) {
	cmd := exec.Command(dockerBinary, "run", "--name", "test", "-d", "busybox", "top")
	out, _, err := runCommandWithOutput(cmd)
	if err != nil {
		c.Fatalf("failed to run container: %v, output: %q", err, out)
	}
	cmd = exec.Command(dockerBinary, "run", "--name", "parent", "-d", "--net=container:test", "busybox", "top")
	out, _, err = runCommandWithOutput(cmd)
	if err != nil {
		c.Fatalf("failed to run container: %v, output: %q", err, out)
	}
	cmd = exec.Command(dockerBinary, "run", "-d", "--link=parent:parent", "busybox", "top")
	out, _, err = runCommandWithOutput(cmd)
	if err != nil {
		c.Fatalf("failed to run container: %v, output: %q", err, out)
	}

	cmd = exec.Command(dockerBinary, "run", "--name", "child", "-d", "--net=container:parent", "busybox", "top")
	out, _, err = runCommandWithOutput(cmd)
	if err != nil {
		c.Fatalf("failed to run container: %v, output: %q", err, out)
	}
	cmd = exec.Command(dockerBinary, "run", "-d", "--link=child:child", "busybox", "top")
	out, _, err = runCommandWithOutput(cmd)
	if err != nil {
		c.Fatalf("failed to run container: %v, output: %q", err, out)
	}
}

func (s *DockerSuite) TestRunContainerNetModeWithDnsMacHosts(c *check.C) {
	cmd := exec.Command(dockerBinary, "run", "-d", "--name", "parent", "busybox", "top")
	out, _, err := runCommandWithOutput(cmd)
	if err != nil {
		c.Fatalf("failed to run container: %v, output: %q", err, out)
	}

	cmd = exec.Command(dockerBinary, "run", "--dns", "1.2.3.4", "--net=container:parent", "busybox")
	out, _, err = runCommandWithOutput(cmd)
	if err == nil || !strings.Contains(out, "Conflicting options: --dns and the network mode") {
		c.Fatalf("run --net=container with --dns should error out")
	}

	cmd = exec.Command(dockerBinary, "run", "--mac-address", "92:d0:c6:0a:29:33", "--net=container:parent", "busybox")
	out, _, err = runCommandWithOutput(cmd)
	if err == nil || !strings.Contains(out, "--mac-address and the network mode") {
		c.Fatalf("run --net=container with --mac-address should error out")
	}

	cmd = exec.Command(dockerBinary, "run", "--add-host", "test:192.168.2.109", "--net=container:parent", "busybox")
	out, _, err = runCommandWithOutput(cmd)
	if err == nil || !strings.Contains(out, "--add-host and the network mode") {
		c.Fatalf("run --net=container with --add-host should error out")
	}

}

func (s *DockerSuite) TestRunModeNetContainerHostname(c *check.C) {
	testRequires(c, ExecSupport)
	cmd := exec.Command(dockerBinary, "run", "-i", "-d", "--name", "parent", "busybox", "top")
	out, _, err := runCommandWithOutput(cmd)
	if err != nil {
		c.Fatalf("failed to run container: %v, output: %q", err, out)
	}
	cmd = exec.Command(dockerBinary, "exec", "parent", "cat", "/etc/hostname")
	out, _, err = runCommandWithOutput(cmd)
	if err != nil {
		c.Fatalf("failed to exec command: %v, output: %q", err, out)
	}

	cmd = exec.Command(dockerBinary, "run", "--net=container:parent", "busybox", "cat", "/etc/hostname")
	out1, _, err := runCommandWithOutput(cmd)
	if err != nil {
		c.Fatalf("failed to run container: %v, output: %q", err, out1)
	}
	if out1 != out {
		c.Fatal("containers with shared net namespace should have same hostname")
	}
}

// Regression test for #4979
func (s *DockerSuite) TestRunWithVolumesFromExited(c *check.C) {
	runCmd := exec.Command(dockerBinary, "run", "--name", "test-data", "--volume", "/some/dir", "busybox", "touch", "/some/dir/file")
	out, stderr, exitCode, err := runCommandWithStdoutStderr(runCmd)
	if err != nil && exitCode != 0 {
		c.Fatal("1", out, stderr, err)
	}

	runCmd = exec.Command(dockerBinary, "run", "--volumes-from", "test-data", "busybox", "cat", "/some/dir/file")
	out, stderr, exitCode, err = runCommandWithStdoutStderr(runCmd)
	if err != nil && exitCode != 0 {
		c.Fatal("2", out, stderr, err)
	}
}

// Volume path is a symlink which also exists on the host, and the host side is a file not a dir
// But the volume call is just a normal volume, not a bind mount
func (s *DockerSuite) TestRunCreateVolumesInSymlinkDir(c *check.C) {
	testRequires(c, SameHostDaemon)
	testRequires(c, NativeExecDriver)
	name := "test-volume-symlink"

	dir, err := ioutil.TempDir("", name)
	if err != nil {
		c.Fatal(err)
	}
	defer os.RemoveAll(dir)

	f, err := os.OpenFile(filepath.Join(dir, "test"), os.O_CREATE, 0700)
	if err != nil {
		c.Fatal(err)
	}
	f.Close()

	dockerFile := fmt.Sprintf("FROM busybox\nRUN mkdir -p %s\nRUN ln -s %s /test", dir, dir)
	if _, err := buildImage(name, dockerFile, false); err != nil {
		c.Fatal(err)
	}

	out, _, err := runCommandWithOutput(exec.Command(dockerBinary, "run", "-v", "/test/test", name))
	if err != nil {
		c.Fatalf("Failed with errors: %s, %v", out, err)
	}
}

func (s *DockerSuite) TestRunVolumesMountedAsReadonly(c *check.C) {
	cmd := exec.Command(dockerBinary, "run", "-v", "/test:/test:ro", "busybox", "touch", "/test/somefile")
	if code, err := runCommand(cmd); err == nil || code == 0 {
		c.Fatalf("run should fail because volume is ro: exit code %d", code)
	}
}

func (s *DockerSuite) TestRunVolumesFromInReadonlyMode(c *check.C) {
	cmd := exec.Command(dockerBinary, "run", "--name", "parent", "-v", "/test", "busybox", "true")
	if _, err := runCommand(cmd); err != nil {
		c.Fatal(err)
	}

	cmd = exec.Command(dockerBinary, "run", "--volumes-from", "parent:ro", "busybox", "touch", "/test/file")
	if code, err := runCommand(cmd); err == nil || code == 0 {
		c.Fatalf("run should fail because volume is ro: exit code %d", code)
	}
}

// Regression test for #1201
func (s *DockerSuite) TestRunVolumesFromInReadWriteMode(c *check.C) {
	cmd := exec.Command(dockerBinary, "run", "--name", "parent", "-v", "/test", "busybox", "true")
	if _, err := runCommand(cmd); err != nil {
		c.Fatal(err)
	}

	cmd = exec.Command(dockerBinary, "run", "--volumes-from", "parent:rw", "busybox", "touch", "/test/file")
	if out, _, err := runCommandWithOutput(cmd); err != nil {
		c.Fatalf("running --volumes-from parent:rw failed with output: %q\nerror: %v", out, err)
	}

	cmd = exec.Command(dockerBinary, "run", "--volumes-from", "parent:bar", "busybox", "touch", "/test/file")
	if out, _, err := runCommandWithOutput(cmd); err == nil || !strings.Contains(out, "invalid mode for volumes-from: bar") {
		c.Fatalf("running --volumes-from foo:bar should have failed with invalid mount mode: %q", out)
	}

	cmd = exec.Command(dockerBinary, "run", "--volumes-from", "parent", "busybox", "touch", "/test/file")
	if out, _, err := runCommandWithOutput(cmd); err != nil {
		c.Fatalf("running --volumes-from parent failed with output: %q\nerror: %v", out, err)
	}
}

func (s *DockerSuite) TestVolumesFromGetsProperMode(c *check.C) {
	cmd := exec.Command(dockerBinary, "run", "--name", "parent", "-v", "/test:/test:ro", "busybox", "true")
	if _, err := runCommand(cmd); err != nil {
		c.Fatal(err)
	}
	// Expect this "rw" mode to be be ignored since the inherited volume is "ro"
	cmd = exec.Command(dockerBinary, "run", "--volumes-from", "parent:rw", "busybox", "touch", "/test/file")
	if _, err := runCommand(cmd); err == nil {
		c.Fatal("Expected volumes-from to inherit read-only volume even when passing in `rw`")
	}

	cmd = exec.Command(dockerBinary, "run", "--name", "parent2", "-v", "/test:/test:ro", "busybox", "true")
	if _, err := runCommand(cmd); err != nil {
		c.Fatal(err)
	}
	// Expect this to be read-only since both are "ro"
	cmd = exec.Command(dockerBinary, "run", "--volumes-from", "parent2:ro", "busybox", "touch", "/test/file")
	if _, err := runCommand(cmd); err == nil {
		c.Fatal("Expected volumes-from to inherit read-only volume even when passing in `ro`")
	}
}

// Test for GH#10618
func (s *DockerSuite) TestRunNoDupVolumes(c *check.C) {
	mountstr1 := randomUnixTmpDirPath("test1") + ":/someplace"
	mountstr2 := randomUnixTmpDirPath("test2") + ":/someplace"

	cmd := exec.Command(dockerBinary, "run", "-v", mountstr1, "-v", mountstr2, "busybox", "true")
	if out, _, err := runCommandWithOutput(cmd); err == nil {
		c.Fatal("Expected error about duplicate volume definitions")
	} else {
		if !strings.Contains(out, "Duplicate bind mount") {
			c.Fatalf("Expected 'duplicate volume' error, got %v", err)
		}
	}
}

// Test for #1351
func (s *DockerSuite) TestRunApplyVolumesFromBeforeVolumes(c *check.C) {
	cmd := exec.Command(dockerBinary, "run", "--name", "parent", "-v", "/test", "busybox", "touch", "/test/foo")
	if _, err := runCommand(cmd); err != nil {
		c.Fatal(err)
	}

	cmd = exec.Command(dockerBinary, "run", "--volumes-from", "parent", "-v", "/test", "busybox", "cat", "/test/foo")
	if out, _, err := runCommandWithOutput(cmd); err != nil {
		c.Fatal(out, err)
	}
}

func (s *DockerSuite) TestRunMultipleVolumesFrom(c *check.C) {
	cmd := exec.Command(dockerBinary, "run", "--name", "parent1", "-v", "/test", "busybox", "touch", "/test/foo")
	if _, err := runCommand(cmd); err != nil {
		c.Fatal(err)
	}

	cmd = exec.Command(dockerBinary, "run", "--name", "parent2", "-v", "/other", "busybox", "touch", "/other/bar")
	if _, err := runCommand(cmd); err != nil {
		c.Fatal(err)
	}

	cmd = exec.Command(dockerBinary, "run", "--volumes-from", "parent1", "--volumes-from", "parent2",
		"busybox", "sh", "-c", "cat /test/foo && cat /other/bar")
	if _, err := runCommand(cmd); err != nil {
		c.Fatal(err)
	}
}

// this tests verifies the ID format for the container
func (s *DockerSuite) TestRunVerifyContainerID(c *check.C) {
	cmd := exec.Command(dockerBinary, "run", "-d", "busybox", "true")
	out, exit, err := runCommandWithOutput(cmd)
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
func (s *DockerSuite) TestRunCreateVolume(c *check.C) {
	cmd := exec.Command(dockerBinary, "run", "-v", "/var/lib/data", "busybox", "true")
	if _, err := runCommand(cmd); err != nil {
		c.Fatal(err)
	}
}

// Test that creating a volume with a symlink in its path works correctly. Test for #5152.
// Note that this bug happens only with symlinks with a target that starts with '/'.
func (s *DockerSuite) TestRunCreateVolumeWithSymlink(c *check.C) {
	image := "docker-test-createvolumewithsymlink"

	buildCmd := exec.Command(dockerBinary, "build", "-t", image, "-")
	buildCmd.Stdin = strings.NewReader(`FROM busybox
		RUN ln -s home /bar`)
	buildCmd.Dir = workingDirectory
	err := buildCmd.Run()
	if err != nil {
		c.Fatalf("could not build '%s': %v", image, err)
	}

	cmd := exec.Command(dockerBinary, "run", "-v", "/bar/foo", "--name", "test-createvolumewithsymlink", image, "sh", "-c", "mount | grep -q /home/foo")
	exitCode, err := runCommand(cmd)
	if err != nil || exitCode != 0 {
		c.Fatalf("[run] err: %v, exitcode: %d", err, exitCode)
	}

	var volPath string
	cmd = exec.Command(dockerBinary, "inspect", "-f", "{{range .Volumes}}{{.}}{{end}}", "test-createvolumewithsymlink")
	volPath, exitCode, err = runCommandWithOutput(cmd)
	if err != nil || exitCode != 0 {
		c.Fatalf("[inspect] err: %v, exitcode: %d", err, exitCode)
	}

	cmd = exec.Command(dockerBinary, "rm", "-v", "test-createvolumewithsymlink")
	exitCode, err = runCommand(cmd)
	if err != nil || exitCode != 0 {
		c.Fatalf("[rm] err: %v, exitcode: %d", err, exitCode)
	}

	f, err := os.Open(volPath)
	defer f.Close()
	if !os.IsNotExist(err) {
		c.Fatalf("[open] (expecting 'file does not exist' error) err: %v, volPath: %s", err, volPath)
	}
}

// Tests that a volume path that has a symlink exists in a container mounting it with `--volumes-from`.
func (s *DockerSuite) TestRunVolumesFromSymlinkPath(c *check.C) {
	name := "docker-test-volumesfromsymlinkpath"

	buildCmd := exec.Command(dockerBinary, "build", "-t", name, "-")
	buildCmd.Stdin = strings.NewReader(`FROM busybox
		RUN ln -s home /foo
		VOLUME ["/foo/bar"]`)
	buildCmd.Dir = workingDirectory
	err := buildCmd.Run()
	if err != nil {
		c.Fatalf("could not build 'docker-test-volumesfromsymlinkpath': %v", err)
	}

	cmd := exec.Command(dockerBinary, "run", "--name", "test-volumesfromsymlinkpath", name)
	exitCode, err := runCommand(cmd)
	if err != nil || exitCode != 0 {
		c.Fatalf("[run] (volume) err: %v, exitcode: %d", err, exitCode)
	}

	cmd = exec.Command(dockerBinary, "run", "--volumes-from", "test-volumesfromsymlinkpath", "busybox", "sh", "-c", "ls /foo | grep -q bar")
	exitCode, err = runCommand(cmd)
	if err != nil || exitCode != 0 {
		c.Fatalf("[run] err: %v, exitcode: %d", err, exitCode)
	}
}

func (s *DockerSuite) TestRunExitCode(c *check.C) {
	cmd := exec.Command(dockerBinary, "run", "busybox", "/bin/sh", "-c", "exit 72")

	exit, err := runCommand(cmd)
	if err == nil {
		c.Fatal("should not have a non nil error")
	}
	if exit != 72 {
		c.Fatalf("expected exit code 72 received %d", exit)
	}
}

func (s *DockerSuite) TestRunUserDefaultsToRoot(c *check.C) {
	cmd := exec.Command(dockerBinary, "run", "busybox", "id")
	out, _, err := runCommandWithOutput(cmd)
	if err != nil {
		c.Fatal(err, out)
	}
	if !strings.Contains(out, "uid=0(root) gid=0(root)") {
		c.Fatalf("expected root user got %s", out)
	}
}

func (s *DockerSuite) TestRunUserByName(c *check.C) {
	cmd := exec.Command(dockerBinary, "run", "-u", "root", "busybox", "id")
	out, _, err := runCommandWithOutput(cmd)
	if err != nil {
		c.Fatal(err, out)
	}
	if !strings.Contains(out, "uid=0(root) gid=0(root)") {
		c.Fatalf("expected root user got %s", out)
	}
}

func (s *DockerSuite) TestRunUserByID(c *check.C) {
	cmd := exec.Command(dockerBinary, "run", "-u", "1", "busybox", "id")
	out, _, err := runCommandWithOutput(cmd)
	if err != nil {
		c.Fatal(err, out)
	}
	if !strings.Contains(out, "uid=1(daemon) gid=1(daemon)") {
		c.Fatalf("expected daemon user got %s", out)
	}
}

func (s *DockerSuite) TestRunUserByIDBig(c *check.C) {
	cmd := exec.Command(dockerBinary, "run", "-u", "2147483648", "busybox", "id")
	out, _, err := runCommandWithOutput(cmd)
	if err == nil {
		c.Fatal("No error, but must be.", out)
	}
	if !strings.Contains(out, "Uids and gids must be in range") {
		c.Fatalf("expected error about uids range, got %s", out)
	}
}

func (s *DockerSuite) TestRunUserByIDNegative(c *check.C) {
	cmd := exec.Command(dockerBinary, "run", "-u", "-1", "busybox", "id")
	out, _, err := runCommandWithOutput(cmd)
	if err == nil {
		c.Fatal("No error, but must be.", out)
	}
	if !strings.Contains(out, "Uids and gids must be in range") {
		c.Fatalf("expected error about uids range, got %s", out)
	}
}

func (s *DockerSuite) TestRunUserByIDZero(c *check.C) {
	cmd := exec.Command(dockerBinary, "run", "-u", "0", "busybox", "id")
	out, _, err := runCommandWithOutput(cmd)
	if err != nil {
		c.Fatal(err, out)
	}
	if !strings.Contains(out, "uid=0(root) gid=0(root) groups=10(wheel)") {
		c.Fatalf("expected daemon user got %s", out)
	}
}

func (s *DockerSuite) TestRunUserNotFound(c *check.C) {
	cmd := exec.Command(dockerBinary, "run", "-u", "notme", "busybox", "id")
	_, err := runCommand(cmd)
	if err == nil {
		c.Fatal("unknown user should cause container to fail")
	}
}

func (s *DockerSuite) TestRunTwoConcurrentContainers(c *check.C) {
	group := sync.WaitGroup{}
	group.Add(2)

	errChan := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func() {
			defer group.Done()
			cmd := exec.Command(dockerBinary, "run", "busybox", "sleep", "2")
			_, err := runCommand(cmd)
			errChan <- err
		}()
	}

	group.Wait()
	close(errChan)

	for err := range errChan {
		c.Assert(err, check.IsNil)
	}
}

func (s *DockerSuite) TestRunEnvironment(c *check.C) {
	cmd := exec.Command(dockerBinary, "run", "-h", "testing", "-e=FALSE=true", "-e=TRUE", "-e=TRICKY", "-e=HOME=", "busybox", "env")
	cmd.Env = append(os.Environ(),
		"TRUE=false",
		"TRICKY=tri\ncky\n",
	)

	out, _, err := runCommandWithOutput(cmd)
	if err != nil {
		c.Fatal(err, out)
	}

	actualEnvLxc := strings.Split(strings.TrimSpace(out), "\n")
	actualEnv := []string{}
	for i := range actualEnvLxc {
		if actualEnvLxc[i] != "container=lxc" {
			actualEnv = append(actualEnv, actualEnvLxc[i])
		}
	}
	sort.Strings(actualEnv)

	goodEnv := []string{
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
		c.Fatalf("Wrong environment: should be %d variables, not: %q\n", len(goodEnv), strings.Join(actualEnv, ", "))
	}
	for i := range goodEnv {
		if actualEnv[i] != goodEnv[i] {
			c.Fatalf("Wrong environment variable: should be %s, not %s", goodEnv[i], actualEnv[i])
		}
	}
}

func (s *DockerSuite) TestRunEnvironmentErase(c *check.C) {
	// Test to make sure that when we use -e on env vars that are
	// not set in our local env that they're removed (if present) in
	// the container

	cmd := exec.Command(dockerBinary, "run", "-e", "FOO", "-e", "HOSTNAME", "busybox", "env")
	cmd.Env = appendBaseEnv([]string{})

	out, _, err := runCommandWithOutput(cmd)
	if err != nil {
		c.Fatal(err, out)
	}

	actualEnvLxc := strings.Split(strings.TrimSpace(out), "\n")
	actualEnv := []string{}
	for i := range actualEnvLxc {
		if actualEnvLxc[i] != "container=lxc" {
			actualEnv = append(actualEnv, actualEnvLxc[i])
		}
	}
	sort.Strings(actualEnv)

	goodEnv := []string{
		"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
		"HOME=/root",
	}
	sort.Strings(goodEnv)
	if len(goodEnv) != len(actualEnv) {
		c.Fatalf("Wrong environment: should be %d variables, not: %q\n", len(goodEnv), strings.Join(actualEnv, ", "))
	}
	for i := range goodEnv {
		if actualEnv[i] != goodEnv[i] {
			c.Fatalf("Wrong environment variable: should be %s, not %s", goodEnv[i], actualEnv[i])
		}
	}
}

func (s *DockerSuite) TestRunEnvironmentOverride(c *check.C) {
	// Test to make sure that when we use -e on env vars that are
	// already in the env that we're overriding them

	cmd := exec.Command(dockerBinary, "run", "-e", "HOSTNAME", "-e", "HOME=/root2", "busybox", "env")
	cmd.Env = appendBaseEnv([]string{"HOSTNAME=bar"})

	out, _, err := runCommandWithOutput(cmd)
	if err != nil {
		c.Fatal(err, out)
	}

	actualEnvLxc := strings.Split(strings.TrimSpace(out), "\n")
	actualEnv := []string{}
	for i := range actualEnvLxc {
		if actualEnvLxc[i] != "container=lxc" {
			actualEnv = append(actualEnv, actualEnvLxc[i])
		}
	}
	sort.Strings(actualEnv)

	goodEnv := []string{
		"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
		"HOME=/root2",
		"HOSTNAME=bar",
	}
	sort.Strings(goodEnv)
	if len(goodEnv) != len(actualEnv) {
		c.Fatalf("Wrong environment: should be %d variables, not: %q\n", len(goodEnv), strings.Join(actualEnv, ", "))
	}
	for i := range goodEnv {
		if actualEnv[i] != goodEnv[i] {
			c.Fatalf("Wrong environment variable: should be %s, not %s", goodEnv[i], actualEnv[i])
		}
	}
}

func (s *DockerSuite) TestRunContainerNetwork(c *check.C) {
	cmd := exec.Command(dockerBinary, "run", "busybox", "ping", "-c", "1", "127.0.0.1")
	if _, err := runCommand(cmd); err != nil {
		c.Fatal(err)
	}
}

// Issue #4681
func (s *DockerSuite) TestRunLoopbackWhenNetworkDisabled(c *check.C) {
	cmd := exec.Command(dockerBinary, "run", "--net=none", "busybox", "ping", "-c", "1", "127.0.0.1")
	if _, err := runCommand(cmd); err != nil {
		c.Fatal(err)
	}
}

func (s *DockerSuite) TestRunNetHostNotAllowedWithLinks(c *check.C) {
	out, _, err := runCommandWithOutput(exec.Command(dockerBinary, "run", "--name", "linked", "busybox", "true"))
	if err != nil {
		c.Fatalf("Failed with errors: %s, %v", out, err)
	}
	cmd := exec.Command(dockerBinary, "run", "--net=host", "--link", "linked:linked", "busybox", "true")
	_, _, err = runCommandWithOutput(cmd)
	if err == nil {
		c.Fatal("Expected error")
	}
}

func (s *DockerSuite) TestRunLoopbackOnlyExistsWhenNetworkingDisabled(c *check.C) {
	cmd := exec.Command(dockerBinary, "run", "--net=none", "busybox", "ip", "-o", "-4", "a", "show", "up")
	out, _, err := runCommandWithOutput(cmd)
	if err != nil {
		c.Fatal(err, out)
	}

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

// #7851 hostname outside container shows FQDN, inside only shortname
// For testing purposes it is not required to set host's hostname directly
// and use "--net=host" (as the original issue submitter did), as the same
// codepath is executed with "docker run -h <hostname>".  Both were manually
// tested, but this testcase takes the simpler path of using "run -h .."
func (s *DockerSuite) TestRunFullHostnameSet(c *check.C) {
	cmd := exec.Command(dockerBinary, "run", "-h", "foo.bar.baz", "busybox", "hostname")
	out, _, err := runCommandWithOutput(cmd)
	if err != nil {
		c.Fatal(err, out)
	}

	if actual := strings.Trim(out, "\r\n"); actual != "foo.bar.baz" {
		c.Fatalf("expected hostname 'foo.bar.baz', received %s", actual)
	}
}

func (s *DockerSuite) TestRunPrivilegedCanMknod(c *check.C) {
	cmd := exec.Command(dockerBinary, "run", "--privileged", "busybox", "sh", "-c", "mknod /tmp/sda b 8 0 && echo ok")
	out, _, err := runCommandWithOutput(cmd)
	if err != nil {
		c.Fatal(err)
	}

	if actual := strings.Trim(out, "\r\n"); actual != "ok" {
		c.Fatalf("expected output ok received %s", actual)
	}
}

func (s *DockerSuite) TestRunUnprivilegedCanMknod(c *check.C) {
	cmd := exec.Command(dockerBinary, "run", "busybox", "sh", "-c", "mknod /tmp/sda b 8 0 && echo ok")
	out, _, err := runCommandWithOutput(cmd)
	if err != nil {
		c.Fatal(err)
	}

	if actual := strings.Trim(out, "\r\n"); actual != "ok" {
		c.Fatalf("expected output ok received %s", actual)
	}
}

func (s *DockerSuite) TestRunCapDropInvalid(c *check.C) {
	cmd := exec.Command(dockerBinary, "run", "--cap-drop=CHPASS", "busybox", "ls")
	out, _, err := runCommandWithOutput(cmd)
	if err == nil {
		c.Fatal(err, out)
	}
}

func (s *DockerSuite) TestRunCapDropCannotMknod(c *check.C) {
	cmd := exec.Command(dockerBinary, "run", "--cap-drop=MKNOD", "busybox", "sh", "-c", "mknod /tmp/sda b 8 0 && echo ok")
	out, _, err := runCommandWithOutput(cmd)
	if err == nil {
		c.Fatal(err, out)
	}

	if actual := strings.Trim(out, "\r\n"); actual == "ok" {
		c.Fatalf("expected output not ok received %s", actual)
	}
}

func (s *DockerSuite) TestRunCapDropCannotMknodLowerCase(c *check.C) {
	cmd := exec.Command(dockerBinary, "run", "--cap-drop=mknod", "busybox", "sh", "-c", "mknod /tmp/sda b 8 0 && echo ok")
	out, _, err := runCommandWithOutput(cmd)
	if err == nil {
		c.Fatal(err, out)
	}

	if actual := strings.Trim(out, "\r\n"); actual == "ok" {
		c.Fatalf("expected output not ok received %s", actual)
	}
}

func (s *DockerSuite) TestRunCapDropALLCannotMknod(c *check.C) {
	cmd := exec.Command(dockerBinary, "run", "--cap-drop=ALL", "--cap-add=SETGID", "busybox", "sh", "-c", "mknod /tmp/sda b 8 0 && echo ok")
	out, _, err := runCommandWithOutput(cmd)
	if err == nil {
		c.Fatal(err, out)
	}

	if actual := strings.Trim(out, "\r\n"); actual == "ok" {
		c.Fatalf("expected output not ok received %s", actual)
	}
}

func (s *DockerSuite) TestRunCapDropALLAddMknodCanMknod(c *check.C) {
	cmd := exec.Command(dockerBinary, "run", "--cap-drop=ALL", "--cap-add=MKNOD", "--cap-add=SETGID", "busybox", "sh", "-c", "mknod /tmp/sda b 8 0 && echo ok")
	out, _, err := runCommandWithOutput(cmd)
	if err != nil {
		c.Fatal(err, out)
	}

	if actual := strings.Trim(out, "\r\n"); actual != "ok" {
		c.Fatalf("expected output ok received %s", actual)
	}
}

func (s *DockerSuite) TestRunCapAddInvalid(c *check.C) {
	cmd := exec.Command(dockerBinary, "run", "--cap-add=CHPASS", "busybox", "ls")
	out, _, err := runCommandWithOutput(cmd)
	if err == nil {
		c.Fatal(err, out)
	}
}

func (s *DockerSuite) TestRunCapAddCanDownInterface(c *check.C) {
	cmd := exec.Command(dockerBinary, "run", "--cap-add=NET_ADMIN", "busybox", "sh", "-c", "ip link set eth0 down && echo ok")
	out, _, err := runCommandWithOutput(cmd)
	if err != nil {
		c.Fatal(err, out)
	}

	if actual := strings.Trim(out, "\r\n"); actual != "ok" {
		c.Fatalf("expected output ok received %s", actual)
	}
}

func (s *DockerSuite) TestRunCapAddALLCanDownInterface(c *check.C) {
	cmd := exec.Command(dockerBinary, "run", "--cap-add=ALL", "busybox", "sh", "-c", "ip link set eth0 down && echo ok")
	out, _, err := runCommandWithOutput(cmd)
	if err != nil {
		c.Fatal(err, out)
	}

	if actual := strings.Trim(out, "\r\n"); actual != "ok" {
		c.Fatalf("expected output ok received %s", actual)
	}
}

func (s *DockerSuite) TestRunCapAddALLDropNetAdminCanDownInterface(c *check.C) {
	cmd := exec.Command(dockerBinary, "run", "--cap-add=ALL", "--cap-drop=NET_ADMIN", "busybox", "sh", "-c", "ip link set eth0 down && echo ok")
	out, _, err := runCommandWithOutput(cmd)
	if err == nil {
		c.Fatal(err, out)
	}

	if actual := strings.Trim(out, "\r\n"); actual == "ok" {
		c.Fatalf("expected output not ok received %s", actual)
	}
}

func (s *DockerSuite) TestRunPrivilegedCanMount(c *check.C) {
	cmd := exec.Command(dockerBinary, "run", "--privileged", "busybox", "sh", "-c", "mount -t tmpfs none /tmp && echo ok")
	out, _, err := runCommandWithOutput(cmd)
	if err != nil {
		c.Fatal(err)
	}

	if actual := strings.Trim(out, "\r\n"); actual != "ok" {
		c.Fatalf("expected output ok received %s", actual)
	}
}

func (s *DockerSuite) TestRunUnprivilegedCannotMount(c *check.C) {
	cmd := exec.Command(dockerBinary, "run", "busybox", "sh", "-c", "mount -t tmpfs none /tmp && echo ok")
	out, _, err := runCommandWithOutput(cmd)
	if err == nil {
		c.Fatal(err, out)
	}

	if actual := strings.Trim(out, "\r\n"); actual == "ok" {
		c.Fatalf("expected output not ok received %s", actual)
	}
}

func (s *DockerSuite) TestRunSysNotWritableInNonPrivilegedContainers(c *check.C) {
	cmd := exec.Command(dockerBinary, "run", "busybox", "touch", "/sys/kernel/profiling")
	if code, err := runCommand(cmd); err == nil || code == 0 {
		c.Fatal("sys should not be writable in a non privileged container")
	}
}

func (s *DockerSuite) TestRunSysWritableInPrivilegedContainers(c *check.C) {
	cmd := exec.Command(dockerBinary, "run", "--privileged", "busybox", "touch", "/sys/kernel/profiling")
	if code, err := runCommand(cmd); err != nil || code != 0 {
		c.Fatalf("sys should be writable in privileged container")
	}
}

func (s *DockerSuite) TestRunProcNotWritableInNonPrivilegedContainers(c *check.C) {
	cmd := exec.Command(dockerBinary, "run", "busybox", "touch", "/proc/sysrq-trigger")
	if code, err := runCommand(cmd); err == nil || code == 0 {
		c.Fatal("proc should not be writable in a non privileged container")
	}
}

func (s *DockerSuite) TestRunProcWritableInPrivilegedContainers(c *check.C) {
	cmd := exec.Command(dockerBinary, "run", "--privileged", "busybox", "touch", "/proc/sysrq-trigger")
	if code, err := runCommand(cmd); err != nil || code != 0 {
		c.Fatalf("proc should be writable in privileged container")
	}
}

func (s *DockerSuite) TestRunWithCpuset(c *check.C) {
	cmd := exec.Command(dockerBinary, "run", "--cpuset", "0", "busybox", "true")
	if code, err := runCommand(cmd); err != nil || code != 0 {
		c.Fatalf("container should run successfully with cpuset of 0: %s", err)
	}
}

func (s *DockerSuite) TestRunWithCpusetCpus(c *check.C) {
	cmd := exec.Command(dockerBinary, "run", "--cpuset-cpus", "0", "busybox", "true")
	if code, err := runCommand(cmd); err != nil || code != 0 {
		c.Fatalf("container should run successfully with cpuset-cpus of 0: %s", err)
	}
}

func (s *DockerSuite) TestRunWithCpusetMems(c *check.C) {
	cmd := exec.Command(dockerBinary, "run", "--cpuset-mems", "0", "busybox", "true")
	if code, err := runCommand(cmd); err != nil || code != 0 {
		c.Fatalf("container should run successfully with cpuset-mems of 0: %s", err)
	}
}

func (s *DockerSuite) TestRunWithBlkioWeight(c *check.C) {
	cmd := exec.Command(dockerBinary, "run", "--blkio-weight", "300", "busybox", "true")
	if code, err := runCommand(cmd); err != nil || code != 0 {
		c.Fatalf("container should run successfully with blkio-weight of 300: %s", err)
	}
}

func (s *DockerSuite) TestRunWithBlkioInvalidWeight(c *check.C) {
	cmd := exec.Command(dockerBinary, "run", "--blkio-weight", "5", "busybox", "true")
	if _, err := runCommand(cmd); err == nil {
		c.Fatalf("run with invalid blkio-weight should failed")
	}
}

func (s *DockerSuite) TestRunDeviceNumbers(c *check.C) {
	cmd := exec.Command(dockerBinary, "run", "busybox", "sh", "-c", "ls -l /dev/null")
	out, _, err := runCommandWithOutput(cmd)
	if err != nil {
		c.Fatal(err, out)
	}
	deviceLineFields := strings.Fields(out)
	deviceLineFields[6] = ""
	deviceLineFields[7] = ""
	deviceLineFields[8] = ""
	expected := []string{"crw-rw-rw-", "1", "root", "root", "1,", "3", "", "", "", "/dev/null"}

	if !(reflect.DeepEqual(deviceLineFields, expected)) {
		c.Fatalf("expected output\ncrw-rw-rw- 1 root root 1, 3 May 24 13:29 /dev/null\n received\n %s\n", out)
	}
}

func (s *DockerSuite) TestRunThatCharacterDevicesActLikeCharacterDevices(c *check.C) {
	cmd := exec.Command(dockerBinary, "run", "busybox", "sh", "-c", "dd if=/dev/zero of=/zero bs=1k count=5 2> /dev/null ; du -h /zero")
	out, _, err := runCommandWithOutput(cmd)
	if err != nil {
		c.Fatal(err, out)
	}

	if actual := strings.Trim(out, "\r\n"); actual[0] == '0' {
		c.Fatalf("expected a new file called /zero to be create that is greater than 0 bytes long, but du says: %s", actual)
	}
}

func (s *DockerSuite) TestRunUnprivilegedWithChroot(c *check.C) {
	cmd := exec.Command(dockerBinary, "run", "busybox", "chroot", "/", "true")
	if _, err := runCommand(cmd); err != nil {
		c.Fatal(err)
	}
}

func (s *DockerSuite) TestRunAddingOptionalDevices(c *check.C) {
	cmd := exec.Command(dockerBinary, "run", "--device", "/dev/zero:/dev/nulo", "busybox", "sh", "-c", "ls /dev/nulo")
	out, _, err := runCommandWithOutput(cmd)
	if err != nil {
		c.Fatal(err, out)
	}

	if actual := strings.Trim(out, "\r\n"); actual != "/dev/nulo" {
		c.Fatalf("expected output /dev/nulo, received %s", actual)
	}
}

func (s *DockerSuite) TestRunModeHostname(c *check.C) {
	testRequires(c, SameHostDaemon)

	cmd := exec.Command(dockerBinary, "run", "-h=testhostname", "busybox", "cat", "/etc/hostname")
	out, _, err := runCommandWithOutput(cmd)
	if err != nil {
		c.Fatal(err, out)
	}

	if actual := strings.Trim(out, "\r\n"); actual != "testhostname" {
		c.Fatalf("expected 'testhostname', but says: %q", actual)
	}

	cmd = exec.Command(dockerBinary, "run", "--net=host", "busybox", "cat", "/etc/hostname")

	out, _, err = runCommandWithOutput(cmd)
	if err != nil {
		c.Fatal(err, out)
	}
	hostname, err := os.Hostname()
	if err != nil {
		c.Fatal(err)
	}
	if actual := strings.Trim(out, "\r\n"); actual != hostname {
		c.Fatalf("expected %q, but says: %q", hostname, actual)
	}
}

func (s *DockerSuite) TestRunRootWorkdir(c *check.C) {
	out, _, err := runCommandWithOutput(exec.Command(dockerBinary, "run", "--workdir", "/", "busybox", "pwd"))
	if err != nil {
		c.Fatalf("Failed with errors: %s, %v", out, err)
	}
	if out != "/\n" {
		c.Fatalf("pwd returned %q (expected /\\n)", s)
	}
}

func (s *DockerSuite) TestRunAllowBindMountingRoot(c *check.C) {
	out, _, err := runCommandWithOutput(exec.Command(dockerBinary, "run", "-v", "/:/host", "busybox", "ls", "/host"))
	if err != nil {
		c.Fatalf("Failed with errors: %s, %v", out, err)
	}
}

func (s *DockerSuite) TestRunDisallowBindMountingRootToRoot(c *check.C) {
	cmd := exec.Command(dockerBinary, "run", "-v", "/:/", "busybox", "ls", "/host")
	out, _, err := runCommandWithOutput(cmd)
	if err == nil {
		c.Fatal(out, err)
	}
}

// Verify that a container gets default DNS when only localhost resolvers exist
func (s *DockerSuite) TestRunDnsDefaultOptions(c *check.C) {
	testRequires(c, SameHostDaemon)

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

	cmd := exec.Command(dockerBinary, "run", "busybox", "cat", "/etc/resolv.conf")

	actual, _, err := runCommandWithOutput(cmd)
	if err != nil {
		c.Fatal(err, actual)
	}

	// check that the actual defaults are appended to the commented out
	// localhost resolver (which should be preserved)
	// NOTE: if we ever change the defaults from google dns, this will break
	expected := "#nameserver 127.0.2.1\n\nnameserver 8.8.8.8\nnameserver 8.8.4.4"
	if actual != expected {
		c.Fatalf("expected resolv.conf be: %q, but was: %q", expected, actual)
	}
}

func (s *DockerSuite) TestRunDnsOptions(c *check.C) {
	cmd := exec.Command(dockerBinary, "run", "--dns=127.0.0.1", "--dns-search=mydomain", "busybox", "cat", "/etc/resolv.conf")

	out, stderr, _, err := runCommandWithStdoutStderr(cmd)
	if err != nil {
		c.Fatal(err, out)
	}

	// The client will get a warning on stderr when setting DNS to a localhost address; verify this:
	if !strings.Contains(stderr, "Localhost DNS setting") {
		c.Fatalf("Expected warning on stderr about localhost resolver, but got %q", stderr)
	}

	actual := strings.Replace(strings.Trim(out, "\r\n"), "\n", " ", -1)
	if actual != "nameserver 127.0.0.1 search mydomain" {
		c.Fatalf("expected 'nameserver 127.0.0.1 search mydomain', but says: %q", actual)
	}

	cmd = exec.Command(dockerBinary, "run", "--dns=127.0.0.1", "--dns-search=.", "busybox", "cat", "/etc/resolv.conf")

	out, _, _, err = runCommandWithStdoutStderr(cmd)
	if err != nil {
		c.Fatal(err, out)
	}

	actual = strings.Replace(strings.Trim(strings.Trim(out, "\r\n"), " "), "\n", " ", -1)
	if actual != "nameserver 127.0.0.1" {
		c.Fatalf("expected 'nameserver 127.0.0.1', but says: %q", actual)
	}
}

func (s *DockerSuite) TestRunDnsOptionsBasedOnHostResolvConf(c *check.C) {
	testRequires(c, SameHostDaemon)

	origResolvConf, err := ioutil.ReadFile("/etc/resolv.conf")
	if os.IsNotExist(err) {
		c.Fatalf("/etc/resolv.conf does not exist")
	}

	hostNamservers := resolvconf.GetNameservers(origResolvConf)
	hostSearch := resolvconf.GetSearchDomains(origResolvConf)

	var out string
	cmd := exec.Command(dockerBinary, "run", "--dns=127.0.0.1", "busybox", "cat", "/etc/resolv.conf")
	if out, _, _, err = runCommandWithStdoutStderr(cmd); err != nil {
		c.Fatal(err, out)
	}

	if actualNameservers := resolvconf.GetNameservers([]byte(out)); string(actualNameservers[0]) != "127.0.0.1" {
		c.Fatalf("expected '127.0.0.1', but says: %q", string(actualNameservers[0]))
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

	cmd = exec.Command(dockerBinary, "run", "--dns-search=mydomain", "busybox", "cat", "/etc/resolv.conf")

	if out, _, err = runCommandWithOutput(cmd); err != nil {
		c.Fatal(err, out)
	}

	actualNameservers := resolvconf.GetNameservers([]byte(out))
	if len(actualNameservers) != len(hostNamservers) {
		c.Fatalf("expected %q nameserver(s), but it has: %q", len(hostNamservers), len(actualNameservers))
	}
	for i := range actualNameservers {
		if actualNameservers[i] != hostNamservers[i] {
			c.Fatalf("expected %q nameserver, but says: %q", actualNameservers[i], hostNamservers[i])
		}
	}

	if actualSearch = resolvconf.GetSearchDomains([]byte(out)); string(actualSearch[0]) != "mydomain" {
		c.Fatalf("expected 'mydomain', but says: %q", string(actualSearch[0]))
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

	hostNamservers = resolvconf.GetNameservers(resolvConf)
	hostSearch = resolvconf.GetSearchDomains(resolvConf)

	cmd = exec.Command(dockerBinary, "run", "busybox", "cat", "/etc/resolv.conf")

	if out, _, err = runCommandWithOutput(cmd); err != nil {
		c.Fatal(err, out)
	}

	if actualNameservers = resolvconf.GetNameservers([]byte(out)); string(actualNameservers[0]) != "12.34.56.78" || len(actualNameservers) != 1 {
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

// Test to see if a non-root user can resolve a DNS name and reach out to it. Also
// check if the container resolv.conf file has atleast 0644 perm.
func (s *DockerSuite) TestRunNonRootUserResolvName(c *check.C) {
	testRequires(c, SameHostDaemon)
	testRequires(c, Network)

	cmd := exec.Command(dockerBinary, "run", "--name=testperm", "--user=default", "busybox", "ping", "-c", "1", "www.docker.io")
	if out, err := runCommand(cmd); err != nil {
		c.Fatal(err, out)
	}

	cID, err := getIDByName("testperm")
	if err != nil {
		c.Fatal(err)
	}

	fmode := (os.FileMode)(0644)
	finfo, err := os.Stat(containerStorageFile(cID, "resolv.conf"))
	if err != nil {
		c.Fatal(err)
	}

	if (finfo.Mode() & fmode) != fmode {
		c.Fatalf("Expected container resolv.conf mode to be atleast %s, instead got %s", fmode.String(), finfo.Mode().String())
	}
}

// Test if container resolv.conf gets updated the next time it restarts
// if host /etc/resolv.conf has changed. This only applies if the container
// uses the host's /etc/resolv.conf and does not have any dns options provided.
func (s *DockerSuite) TestRunResolvconfUpdate(c *check.C) {
	testRequires(c, SameHostDaemon)

	tmpResolvConf := []byte("search pommesfrites.fr\nnameserver 12.34.56.78")
	tmpLocalhostResolvConf := []byte("nameserver 127.0.0.1")

	//take a copy of resolv.conf for restoring after test completes
	resolvConfSystem, err := ioutil.ReadFile("/etc/resolv.conf")
	if err != nil {
		c.Fatal(err)
	}

	// This test case is meant to test monitoring resolv.conf when it is
	// a regular file not a bind mounc. So we unmount resolv.conf and replace
	// it with a file containing the original settings.
	cmd := exec.Command("umount", "/etc/resolv.conf")
	if _, err = runCommand(cmd); err != nil {
		c.Fatal(err)
	}

	//cleanup
	defer func() {
		if err := ioutil.WriteFile("/etc/resolv.conf", resolvConfSystem, 0644); err != nil {
			c.Fatal(err)
		}
	}()

	//1. test that a restarting container gets an updated resolv.conf
	cmd = exec.Command(dockerBinary, "run", "--name='first'", "busybox", "true")
	if _, err := runCommand(cmd); err != nil {
		c.Fatal(err)
	}
	containerID1, err := getIDByName("first")
	if err != nil {
		c.Fatal(err)
	}

	// replace resolv.conf with our temporary copy
	bytesResolvConf := []byte(tmpResolvConf)
	if err := ioutil.WriteFile("/etc/resolv.conf", bytesResolvConf, 0644); err != nil {
		c.Fatal(err)
	}

	// start the container again to pickup changes
	cmd = exec.Command(dockerBinary, "start", "first")
	if out, err := runCommand(cmd); err != nil {
		c.Fatalf("Errored out %s, \nerror: %v", string(out), err)
	}

	// check for update in container
	containerResolv, err := readContainerFile(containerID1, "resolv.conf")
	if err != nil {
		c.Fatal(err)
	}
	if !bytes.Equal(containerResolv, bytesResolvConf) {
		c.Fatalf("Restarted container does not have updated resolv.conf; expected %q, got %q", tmpResolvConf, string(containerResolv))
	}

	/* 	//make a change to resolv.conf (in this case replacing our tmp copy with orig copy)
	   	if err := ioutil.WriteFile("/etc/resolv.conf", resolvConfSystem, 0644); err != nil {
	   		c.Fatal(err)
	   	} */
	//2. test that a restarting container does not receive resolv.conf updates
	//   if it modified the container copy of the starting point resolv.conf
	cmd = exec.Command(dockerBinary, "run", "--name='second'", "busybox", "sh", "-c", "echo 'search mylittlepony.com' >>/etc/resolv.conf")
	if _, err = runCommand(cmd); err != nil {
		c.Fatal(err)
	}
	containerID2, err := getIDByName("second")
	if err != nil {
		c.Fatal(err)
	}

	//make a change to resolv.conf (in this case replacing our tmp copy with orig copy)
	if err := ioutil.WriteFile("/etc/resolv.conf", resolvConfSystem, 0644); err != nil {
		c.Fatal(err)
	}

	// start the container again
	cmd = exec.Command(dockerBinary, "start", "second")
	if out, err := runCommand(cmd); err != nil {
		c.Fatalf("Errored out %s, \nerror: %v", string(out), err)
	}

	// check for update in container
	containerResolv, err = readContainerFile(containerID2, "resolv.conf")
	if err != nil {
		c.Fatal(err)
	}

	if bytes.Equal(containerResolv, resolvConfSystem) {
		c.Fatalf("Restarting  a container after container updated resolv.conf should not pick up host changes; expected %q, got %q", string(containerResolv), string(resolvConfSystem))
	}

	//3. test that a running container's resolv.conf is not modified while running
	cmd = exec.Command(dockerBinary, "run", "-d", "busybox", "top")
	out, _, err := runCommandWithOutput(cmd)
	if err != nil {
		c.Fatal(err)
	}
	runningContainerID := strings.TrimSpace(out)

	// replace resolv.conf
	if err := ioutil.WriteFile("/etc/resolv.conf", bytesResolvConf, 0644); err != nil {
		c.Fatal(err)
	}

	// check for update in container
	containerResolv, err = readContainerFile(runningContainerID, "resolv.conf")
	if err != nil {
		c.Fatal(err)
	}

	if bytes.Equal(containerResolv, bytesResolvConf) {
		c.Fatalf("Running container should not have updated resolv.conf; expected %q, got %q", string(resolvConfSystem), string(containerResolv))
	}

	//4. test that a running container's resolv.conf is updated upon restart
	//   (the above container is still running..)
	cmd = exec.Command(dockerBinary, "restart", runningContainerID)
	if _, err = runCommand(cmd); err != nil {
		c.Fatal(err)
	}

	// check for update in container
	containerResolv, err = readContainerFile(runningContainerID, "resolv.conf")
	if err != nil {
		c.Fatal(err)
	}
	if !bytes.Equal(containerResolv, bytesResolvConf) {
		c.Fatalf("Restarted container should have updated resolv.conf; expected %q, got %q", string(bytesResolvConf), string(containerResolv))
	}

	//5. test that additions of a localhost resolver are cleaned from
	//   host resolv.conf before updating container's resolv.conf copies

	// replace resolv.conf with a localhost-only nameserver copy
	bytesResolvConf = []byte(tmpLocalhostResolvConf)
	if err = ioutil.WriteFile("/etc/resolv.conf", bytesResolvConf, 0644); err != nil {
		c.Fatal(err)
	}

	// start the container again to pickup changes
	cmd = exec.Command(dockerBinary, "start", "first")
	if out, err := runCommand(cmd); err != nil {
		c.Fatalf("Errored out %s, \nerror: %v", string(out), err)
	}

	// our first exited container ID should have been updated, but with default DNS
	// after the cleanup of resolv.conf found only a localhost nameserver:
	containerResolv, err = readContainerFile(containerID1, "resolv.conf")
	if err != nil {
		c.Fatal(err)
	}

	expected := "\nnameserver 8.8.8.8\nnameserver 8.8.4.4"
	if !bytes.Equal(containerResolv, []byte(expected)) {
		c.Fatalf("Container does not have cleaned/replaced DNS in resolv.conf; expected %q, got %q", expected, string(containerResolv))
	}

	//6. Test that replacing (as opposed to modifying) resolv.conf triggers an update
	//   of containers' resolv.conf.

	// Restore the original resolv.conf
	if err := ioutil.WriteFile("/etc/resolv.conf", resolvConfSystem, 0644); err != nil {
		c.Fatal(err)
	}

	// Run the container so it picks up the old settings
	cmd = exec.Command(dockerBinary, "run", "--name='third'", "busybox", "true")
	if _, err := runCommand(cmd); err != nil {
		c.Fatal(err)
	}
	containerID3, err := getIDByName("third")
	if err != nil {
		c.Fatal(err)
	}

	// Create a modified resolv.conf.aside and override resolv.conf with it
	bytesResolvConf = []byte(tmpResolvConf)
	if err := ioutil.WriteFile("/etc/resolv.conf.aside", bytesResolvConf, 0644); err != nil {
		c.Fatal(err)
	}

	err = os.Rename("/etc/resolv.conf.aside", "/etc/resolv.conf")
	if err != nil {
		c.Fatal(err)
	}

	// start the container again to pickup changes
	cmd = exec.Command(dockerBinary, "start", "third")
	if out, err := runCommand(cmd); err != nil {
		c.Fatalf("Errored out %s, \nerror: %v", string(out), err)
	}

	// check for update in container
	containerResolv, err = readContainerFile(containerID3, "resolv.conf")
	if err != nil {
		c.Fatal(err)
	}
	if !bytes.Equal(containerResolv, bytesResolvConf) {
		c.Fatalf("Stopped container does not have updated resolv.conf; expected\n%q\n got\n%q", tmpResolvConf, string(containerResolv))
	}

	//cleanup, restore original resolv.conf happens in defer func()
}

func (s *DockerSuite) TestRunAddHost(c *check.C) {
	cmd := exec.Command(dockerBinary, "run", "--add-host=extra:86.75.30.9", "busybox", "grep", "extra", "/etc/hosts")

	out, _, err := runCommandWithOutput(cmd)
	if err != nil {
		c.Fatal(err, out)
	}

	actual := strings.Trim(out, "\r\n")
	if actual != "86.75.30.9\textra" {
		c.Fatalf("expected '86.75.30.9\textra', but says: %q", actual)
	}
}

// Regression test for #6983
func (s *DockerSuite) TestRunAttachStdErrOnlyTTYMode(c *check.C) {
	cmd := exec.Command(dockerBinary, "run", "-t", "-a", "stderr", "busybox", "true")
	exitCode, err := runCommand(cmd)
	if err != nil {
		c.Fatal(err)
	} else if exitCode != 0 {
		c.Fatalf("Container should have exited with error code 0")
	}
}

// Regression test for #6983
func (s *DockerSuite) TestRunAttachStdOutOnlyTTYMode(c *check.C) {
	cmd := exec.Command(dockerBinary, "run", "-t", "-a", "stdout", "busybox", "true")

	exitCode, err := runCommand(cmd)
	if err != nil {
		c.Fatal(err)
	} else if exitCode != 0 {
		c.Fatalf("Container should have exited with error code 0")
	}
}

// Regression test for #6983
func (s *DockerSuite) TestRunAttachStdOutAndErrTTYMode(c *check.C) {
	cmd := exec.Command(dockerBinary, "run", "-t", "-a", "stdout", "-a", "stderr", "busybox", "true")
	exitCode, err := runCommand(cmd)
	if err != nil {
		c.Fatal(err)
	} else if exitCode != 0 {
		c.Fatalf("Container should have exited with error code 0")
	}
}

// Test for #10388 - this will run the same test as TestRunAttachStdOutAndErrTTYMode
// but using --attach instead of -a to make sure we read the flag correctly
func (s *DockerSuite) TestRunAttachWithDetach(c *check.C) {
	cmd := exec.Command(dockerBinary, "run", "-d", "--attach", "stdout", "busybox", "true")
	_, stderr, _, err := runCommandWithStdoutStderr(cmd)
	if err == nil {
		c.Fatal("Container should have exited with error code different than 0")
	} else if !strings.Contains(stderr, "Conflicting options: -a and -d") {
		c.Fatal("Should have been returned an error with conflicting options -a and -d")
	}
}

func (s *DockerSuite) TestRunState(c *check.C) {
	cmd := exec.Command(dockerBinary, "run", "-d", "busybox", "top")

	out, _, err := runCommandWithOutput(cmd)
	if err != nil {
		c.Fatal(err, out)
	}
	id := strings.TrimSpace(out)
	state, err := inspectField(id, "State.Running")
	c.Assert(err, check.IsNil)
	if state != "true" {
		c.Fatal("Container state is 'not running'")
	}
	pid1, err := inspectField(id, "State.Pid")
	c.Assert(err, check.IsNil)
	if pid1 == "0" {
		c.Fatal("Container state Pid 0")
	}

	cmd = exec.Command(dockerBinary, "stop", id)
	out, _, err = runCommandWithOutput(cmd)
	if err != nil {
		c.Fatal(err, out)
	}
	state, err = inspectField(id, "State.Running")
	c.Assert(err, check.IsNil)
	if state != "false" {
		c.Fatal("Container state is 'running'")
	}
	pid2, err := inspectField(id, "State.Pid")
	c.Assert(err, check.IsNil)
	if pid2 == pid1 {
		c.Fatalf("Container state Pid %s, but expected %s", pid2, pid1)
	}

	cmd = exec.Command(dockerBinary, "start", id)
	out, _, err = runCommandWithOutput(cmd)
	if err != nil {
		c.Fatal(err, out)
	}
	state, err = inspectField(id, "State.Running")
	c.Assert(err, check.IsNil)
	if state != "true" {
		c.Fatal("Container state is 'not running'")
	}
	pid3, err := inspectField(id, "State.Pid")
	c.Assert(err, check.IsNil)
	if pid3 == pid1 {
		c.Fatalf("Container state Pid %s, but expected %s", pid2, pid1)
	}
}

// Test for #1737
func (s *DockerSuite) TestRunCopyVolumeUidGid(c *check.C) {
	name := "testrunvolumesuidgid"
	_, err := buildImage(name,
		`FROM busybox
		RUN echo 'dockerio:x:1001:1001::/bin:/bin/false' >> /etc/passwd
		RUN echo 'dockerio:x:1001:' >> /etc/group
		RUN mkdir -p /hello && touch /hello/test && chown dockerio.dockerio /hello`,
		true)
	if err != nil {
		c.Fatal(err)
	}

	// Test that the uid and gid is copied from the image to the volume
	cmd := exec.Command(dockerBinary, "run", "--rm", "-v", "/hello", name, "sh", "-c", "ls -l / | grep hello | awk '{print $3\":\"$4}'")
	out, _, err := runCommandWithOutput(cmd)
	if err != nil {
		c.Fatal(err, out)
	}
	out = strings.TrimSpace(out)
	if out != "dockerio:dockerio" {
		c.Fatalf("Wrong /hello ownership: %s, expected dockerio:dockerio", out)
	}
}

// Test for #1582
func (s *DockerSuite) TestRunCopyVolumeContent(c *check.C) {
	name := "testruncopyvolumecontent"
	_, err := buildImage(name,
		`FROM busybox
		RUN mkdir -p /hello/local && echo hello > /hello/local/world`,
		true)
	if err != nil {
		c.Fatal(err)
	}

	// Test that the content is copied from the image to the volume
	cmd := exec.Command(dockerBinary, "run", "--rm", "-v", "/hello", name, "find", "/hello")
	out, _, err := runCommandWithOutput(cmd)
	if err != nil {
		c.Fatal(err, out)
	}
	if !(strings.Contains(out, "/hello/local/world") && strings.Contains(out, "/hello/local")) {
		c.Fatal("Container failed to transfer content to volume")
	}
}

func (s *DockerSuite) TestRunCleanupCmdOnEntrypoint(c *check.C) {
	name := "testrunmdcleanuponentrypoint"
	if _, err := buildImage(name,
		`FROM busybox
		ENTRYPOINT ["echo"]
        CMD ["testingpoint"]`,
		true); err != nil {
		c.Fatal(err)
	}
	runCmd := exec.Command(dockerBinary, "run", "--entrypoint", "whoami", name)
	out, exit, err := runCommandWithOutput(runCmd)
	if err != nil {
		c.Fatalf("Error: %v, out: %q", err, out)
	}
	if exit != 0 {
		c.Fatalf("expected exit code 0 received %d, out: %q", exit, out)
	}
	out = strings.TrimSpace(out)
	if out != "root" {
		c.Fatalf("Expected output root, got %q", out)
	}
}

// TestRunWorkdirExistsAndIsFile checks that if 'docker run -w' with existing file can be detected
func (s *DockerSuite) TestRunWorkdirExistsAndIsFile(c *check.C) {
	runCmd := exec.Command(dockerBinary, "run", "-w", "/bin/cat", "busybox")
	out, exit, err := runCommandWithOutput(runCmd)
	if !(err != nil && exit == 1 && strings.Contains(out, "Cannot mkdir: /bin/cat is not a directory")) {
		c.Fatalf("Docker must complains about making dir, but we got out: %s, exit: %d, err: %s", out, exit, err)
	}
}

func (s *DockerSuite) TestRunExitOnStdinClose(c *check.C) {
	name := "testrunexitonstdinclose"
	runCmd := exec.Command(dockerBinary, "run", "--name", name, "-i", "busybox", "/bin/cat")

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
	finish := make(chan error)
	go func() {
		finish <- runCmd.Wait()
		close(finish)
	}()
	select {
	case err := <-finish:
		c.Assert(err, check.IsNil)
	case <-time.After(1 * time.Second):
		c.Fatal("docker run failed to exit on stdin close")
	}
	state, err := inspectField(name, "State.Running")
	c.Assert(err, check.IsNil)

	if state != "false" {
		c.Fatal("Container must be stopped after stdin closing")
	}
}

// Test for #2267
func (s *DockerSuite) TestRunWriteHostsFileAndNotCommit(c *check.C) {
	name := "writehosts"
	cmd := exec.Command(dockerBinary, "run", "--name", name, "busybox", "sh", "-c", "echo test2267 >> /etc/hosts && cat /etc/hosts")
	out, _, err := runCommandWithOutput(cmd)
	if err != nil {
		c.Fatal(err, out)
	}
	if !strings.Contains(out, "test2267") {
		c.Fatal("/etc/hosts should contain 'test2267'")
	}

	cmd = exec.Command(dockerBinary, "diff", name)
	if err != nil {
		c.Fatal(err, out)
	}
	out, _, err = runCommandWithOutput(cmd)
	if err != nil {
		c.Fatal(err, out)
	}

	if len(strings.Trim(out, "\r\n")) != 0 && !eqToBaseDiff(out, c) {
		c.Fatal("diff should be empty")
	}
}

func eqToBaseDiff(out string, c *check.C) bool {
	cmd := exec.Command(dockerBinary, "run", "-d", "busybox", "echo", "hello")
	out1, _, err := runCommandWithOutput(cmd)
	cID := strings.TrimSpace(out1)
	cmd = exec.Command(dockerBinary, "diff", cID)
	baseDiff, _, err := runCommandWithOutput(cmd)
	if err != nil {
		c.Fatal(err, baseDiff)
	}
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

// Test for #2267
func (s *DockerSuite) TestRunWriteHostnameFileAndNotCommit(c *check.C) {
	name := "writehostname"
	cmd := exec.Command(dockerBinary, "run", "--name", name, "busybox", "sh", "-c", "echo test2267 >> /etc/hostname && cat /etc/hostname")
	out, _, err := runCommandWithOutput(cmd)
	if err != nil {
		c.Fatal(err, out)
	}
	if !strings.Contains(out, "test2267") {
		c.Fatal("/etc/hostname should contain 'test2267'")
	}

	cmd = exec.Command(dockerBinary, "diff", name)
	if err != nil {
		c.Fatal(err, out)
	}
	out, _, err = runCommandWithOutput(cmd)
	if err != nil {
		c.Fatal(err, out)
	}
	if len(strings.Trim(out, "\r\n")) != 0 && !eqToBaseDiff(out, c) {
		c.Fatal("diff should be empty")
	}
}

// Test for #2267
func (s *DockerSuite) TestRunWriteResolvFileAndNotCommit(c *check.C) {
	name := "writeresolv"
	cmd := exec.Command(dockerBinary, "run", "--name", name, "busybox", "sh", "-c", "echo test2267 >> /etc/resolv.conf && cat /etc/resolv.conf")
	out, _, err := runCommandWithOutput(cmd)
	if err != nil {
		c.Fatal(err, out)
	}
	if !strings.Contains(out, "test2267") {
		c.Fatal("/etc/resolv.conf should contain 'test2267'")
	}

	cmd = exec.Command(dockerBinary, "diff", name)
	if err != nil {
		c.Fatal(err, out)
	}
	out, _, err = runCommandWithOutput(cmd)
	if err != nil {
		c.Fatal(err, out)
	}
	if len(strings.Trim(out, "\r\n")) != 0 && !eqToBaseDiff(out, c) {
		c.Fatal("diff should be empty")
	}
}

func (s *DockerSuite) TestRunWithBadDevice(c *check.C) {
	name := "baddevice"
	cmd := exec.Command(dockerBinary, "run", "--name", name, "--device", "/etc", "busybox", "true")
	out, _, err := runCommandWithOutput(cmd)
	if err == nil {
		c.Fatal("Run should fail with bad device")
	}
	expected := `"/etc": not a device node`
	if !strings.Contains(out, expected) {
		c.Fatalf("Output should contain %q, actual out: %q", expected, out)
	}
}

func (s *DockerSuite) TestRunEntrypoint(c *check.C) {
	name := "entrypoint"
	cmd := exec.Command(dockerBinary, "run", "--name", name, "--entrypoint", "/bin/echo", "busybox", "-n", "foobar")
	out, _, err := runCommandWithOutput(cmd)
	if err != nil {
		c.Fatal(err, out)
	}
	expected := "foobar"
	if out != expected {
		c.Fatalf("Output should be %q, actual out: %q", expected, out)
	}
}

func (s *DockerSuite) TestRunBindMounts(c *check.C) {
	testRequires(c, SameHostDaemon)

	tmpDir, err := ioutil.TempDir("", "docker-test-container")
	if err != nil {
		c.Fatal(err)
	}

	defer os.RemoveAll(tmpDir)
	writeFile(path.Join(tmpDir, "touch-me"), "", c)

	// Test reading from a read-only bind mount
	cmd := exec.Command(dockerBinary, "run", "-v", fmt.Sprintf("%s:/tmp:ro", tmpDir), "busybox", "ls", "/tmp")
	out, _, err := runCommandWithOutput(cmd)
	if err != nil {
		c.Fatal(err, out)
	}
	if !strings.Contains(out, "touch-me") {
		c.Fatal("Container failed to read from bind mount")
	}

	// test writing to bind mount
	cmd = exec.Command(dockerBinary, "run", "-v", fmt.Sprintf("%s:/tmp:rw", tmpDir), "busybox", "touch", "/tmp/holla")
	out, _, err = runCommandWithOutput(cmd)
	if err != nil {
		c.Fatal(err, out)
	}
	readFile(path.Join(tmpDir, "holla"), c) // Will fail if the file doesn't exist

	// test mounting to an illegal destination directory
	cmd = exec.Command(dockerBinary, "run", "-v", fmt.Sprintf("%s:.", tmpDir), "busybox", "ls", ".")
	_, err = runCommand(cmd)
	if err == nil {
		c.Fatal("Container bind mounted illegal directory")
	}

	// test mount a file
	cmd = exec.Command(dockerBinary, "run", "-v", fmt.Sprintf("%s/holla:/tmp/holla:rw", tmpDir), "busybox", "sh", "-c", "echo -n 'yotta' > /tmp/holla")
	_, err = runCommand(cmd)
	if err != nil {
		c.Fatal(err, out)
	}
	content := readFile(path.Join(tmpDir, "holla"), c) // Will fail if the file doesn't exist
	expected := "yotta"
	if content != expected {
		c.Fatalf("Output should be %q, actual out: %q", expected, content)
	}
}

// Ensure that CIDFile gets deleted if it's empty
// Perform this test by making `docker run` fail
func (s *DockerSuite) TestRunCidFileCleanupIfEmpty(c *check.C) {
	tmpDir, err := ioutil.TempDir("", "TestRunCidFile")
	if err != nil {
		c.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)
	tmpCidFile := path.Join(tmpDir, "cid")
	cmd := exec.Command(dockerBinary, "run", "--cidfile", tmpCidFile, "emptyfs")
	out, _, err := runCommandWithOutput(cmd)
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
//sudo docker run --cidfile /tmp/docker_tesc.cid ubuntu echo "test"
// TestRunCidFile tests that run --cidfile returns the longid
func (s *DockerSuite) TestRunCidFileCheckIDLength(c *check.C) {
	tmpDir, err := ioutil.TempDir("", "TestRunCidFile")
	if err != nil {
		c.Fatal(err)
	}
	tmpCidFile := path.Join(tmpDir, "cid")
	defer os.RemoveAll(tmpDir)
	cmd := exec.Command(dockerBinary, "run", "-d", "--cidfile", tmpCidFile, "busybox", "true")
	out, _, err := runCommandWithOutput(cmd)
	if err != nil {
		c.Fatal(err)
	}
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

func (s *DockerSuite) TestRunNetworkNotInitializedNoneMode(c *check.C) {
	cmd := exec.Command(dockerBinary, "run", "-d", "--net=none", "busybox", "top")
	out, _, err := runCommandWithOutput(cmd)
	if err != nil {
		c.Fatal(err)
	}
	id := strings.TrimSpace(out)
	res, err := inspectField(id, "NetworkSettings.IPAddress")
	c.Assert(err, check.IsNil)
	if res != "" {
		c.Fatalf("For 'none' mode network must not be initialized, but container got IP: %s", res)
	}
}

func (s *DockerSuite) TestRunSetMacAddress(c *check.C) {
	mac := "12:34:56:78:9a:bc"

	cmd := exec.Command(dockerBinary, "run", "-i", "--rm", fmt.Sprintf("--mac-address=%s", mac), "busybox", "/bin/sh", "-c", "ip link show eth0 | tail -1 | awk '{print $2}'")
	out, ec, err := runCommandWithOutput(cmd)
	if err != nil {
		c.Fatalf("exec failed:\nexit code=%v\noutput=%s", ec, out)
	}
	actualMac := strings.TrimSpace(out)
	if actualMac != mac {
		c.Fatalf("Set MAC address with --mac-address failed. The container has an incorrect MAC address: %q, expected: %q", actualMac, mac)
	}
}

func (s *DockerSuite) TestRunInspectMacAddress(c *check.C) {
	mac := "12:34:56:78:9a:bc"
	cmd := exec.Command(dockerBinary, "run", "-d", "--mac-address="+mac, "busybox", "top")
	out, _, err := runCommandWithOutput(cmd)
	if err != nil {
		c.Fatal(err)
	}
	id := strings.TrimSpace(out)
	inspectedMac, err := inspectField(id, "NetworkSettings.MacAddress")
	c.Assert(err, check.IsNil)
	if inspectedMac != mac {
		c.Fatalf("docker inspect outputs wrong MAC address: %q, should be: %q", inspectedMac, mac)
	}
}

// test docker run use a invalid mac address
func (s *DockerSuite) TestRunWithInvalidMacAddress(c *check.C) {
	runCmd := exec.Command(dockerBinary, "run", "--mac-address", "92:d0:c6:0a:29", "busybox")
	out, _, err := runCommandWithOutput(runCmd)
	//use a invalid mac address should with a error out
	if err == nil || !strings.Contains(out, "is not a valid mac address") {
		c.Fatalf("run with an invalid --mac-address should with error out")
	}
}

func (s *DockerSuite) TestRunDeallocatePortOnMissingIptablesRule(c *check.C) {
	testRequires(c, SameHostDaemon)

	cmd := exec.Command(dockerBinary, "run", "-d", "-p", "23:23", "busybox", "top")
	out, _, err := runCommandWithOutput(cmd)
	if err != nil {
		c.Fatal(err)
	}
	id := strings.TrimSpace(out)
	ip, err := inspectField(id, "NetworkSettings.IPAddress")
	c.Assert(err, check.IsNil)
	iptCmd := exec.Command("iptables", "-D", "DOCKER", "-d", fmt.Sprintf("%s/32", ip),
		"!", "-i", "docker0", "-o", "docker0", "-p", "tcp", "-m", "tcp", "--dport", "23", "-j", "ACCEPT")
	out, _, err = runCommandWithOutput(iptCmd)
	if err != nil {
		c.Fatal(err, out)
	}
	if err := deleteContainer(id); err != nil {
		c.Fatal(err)
	}
	cmd = exec.Command(dockerBinary, "run", "-d", "-p", "23:23", "busybox", "top")
	out, _, err = runCommandWithOutput(cmd)
	if err != nil {
		c.Fatal(err, out)
	}
}

func (s *DockerSuite) TestRunPortInUse(c *check.C) {
	testRequires(c, SameHostDaemon)

	port := "1234"
	cmd := exec.Command(dockerBinary, "run", "-d", "-p", port+":80", "busybox", "top")
	out, _, err := runCommandWithOutput(cmd)
	if err != nil {
		c.Fatalf("Fail to run listening container")
	}

	cmd = exec.Command(dockerBinary, "run", "-d", "-p", port+":80", "busybox", "top")
	out, _, err = runCommandWithOutput(cmd)
	if err == nil {
		c.Fatalf("Binding on used port must fail")
	}
	if !strings.Contains(out, "port is already allocated") {
		c.Fatalf("Out must be about \"port is already allocated\", got %s", out)
	}
}

// https://github.com/docker/docker/issues/12148
func (s *DockerSuite) TestRunAllocatePortInReservedRange(c *check.C) {
	// allocate a dynamic port to get the most recent
	cmd := exec.Command(dockerBinary, "run", "-d", "-P", "-p", "80", "busybox", "top")
	out, _, err := runCommandWithOutput(cmd)
	if err != nil {
		c.Fatalf("Failed to run, output: %s, error: %s", out, err)
	}
	id := strings.TrimSpace(out)

	cmd = exec.Command(dockerBinary, "port", id, "80")
	out, _, err = runCommandWithOutput(cmd)
	if err != nil {
		c.Fatalf("Failed to get port, output: %s, error: %s", out, err)
	}
	strPort := strings.Split(strings.TrimSpace(out), ":")[1]
	port, err := strconv.ParseInt(strPort, 10, 64)
	if err != nil {
		c.Fatalf("invalid port, got: %s, error: %s", strPort, err)
	}

	// allocate a static port and a dynamic port together, with static port
	// takes the next recent port in dynamic port range.
	cmd = exec.Command(dockerBinary, "run", "-d", "-P",
		"-p", "80",
		"-p", fmt.Sprintf("%d:8080", port+1),
		"busybox", "top")
	out, _, err = runCommandWithOutput(cmd)
	if err != nil {
		c.Fatalf("Failed to run, output: %s, error: %s", out, err)
	}
}

// Regression test for #7792
func (s *DockerSuite) TestRunMountOrdering(c *check.C) {
	testRequires(c, SameHostDaemon)

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

	cmd := exec.Command(dockerBinary, "run",
		"-v", fmt.Sprintf("%s:/tmp", tmpDir),
		"-v", fmt.Sprintf("%s:/tmp/foo", fooDir),
		"-v", fmt.Sprintf("%s:/tmp/tmp2", tmpDir2),
		"-v", fmt.Sprintf("%s:/tmp/tmp2/foo", fooDir),
		"busybox:latest", "sh", "-c",
		"ls /tmp/touch-me && ls /tmp/foo/touch-me && ls /tmp/tmp2/touch-me && ls /tmp/tmp2/foo/touch-me")
	out, _, err := runCommandWithOutput(cmd)
	if err != nil {
		c.Fatal(out, err)
	}
}

// Regression test for https://github.com/docker/docker/issues/8259
func (s *DockerSuite) TestRunReuseBindVolumeThatIsSymlink(c *check.C) {
	testRequires(c, SameHostDaemon)

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
	cmd := exec.Command(dockerBinary, "run", "-v", fmt.Sprintf("%s:/tmp/test", linkPath), "busybox", "ls", "-lh", "/tmp/test")
	if _, err := runCommand(cmd); err != nil {
		c.Fatal(err)
	}

	// Create second container with same symlinked path
	// This will fail if the referenced issue is hit with a "Volume exists" error
	cmd = exec.Command(dockerBinary, "run", "-v", fmt.Sprintf("%s:/tmp/test", linkPath), "busybox", "ls", "-lh", "/tmp/test")
	if out, _, err := runCommandWithOutput(cmd); err != nil {
		c.Fatal(err, out)
	}
}

//GH#10604: Test an "/etc" volume doesn't overlay special bind mounts in container
func (s *DockerSuite) TestRunCreateVolumeEtc(c *check.C) {
	cmd := exec.Command(dockerBinary, "run", "--dns=127.0.0.1", "-v", "/etc", "busybox", "cat", "/etc/resolv.conf")
	out, _, err := runCommandWithOutput(cmd)
	if err != nil {
		c.Fatalf("failed to run container: %v, output: %q", err, out)
	}
	if !strings.Contains(out, "nameserver 127.0.0.1") {
		c.Fatal("/etc volume mount hides /etc/resolv.conf")
	}

	cmd = exec.Command(dockerBinary, "run", "-h=test123", "-v", "/etc", "busybox", "cat", "/etc/hostname")
	out, _, err = runCommandWithOutput(cmd)
	if err != nil {
		c.Fatalf("failed to run container: %v, output: %q", err, out)
	}
	if !strings.Contains(out, "test123") {
		c.Fatal("/etc volume mount hides /etc/hostname")
	}

	cmd = exec.Command(dockerBinary, "run", "--add-host=test:192.168.0.1", "-v", "/etc", "busybox", "cat", "/etc/hosts")
	out, _, err = runCommandWithOutput(cmd)
	if err != nil {
		c.Fatalf("failed to run container: %v, output: %q", err, out)
	}
	out = strings.Replace(out, "\n", " ", -1)
	if !strings.Contains(out, "192.168.0.1\ttest") || !strings.Contains(out, "127.0.0.1\tlocalhost") {
		c.Fatal("/etc volume mount hides /etc/hosts")
	}
}

func (s *DockerSuite) TestVolumesNoCopyData(c *check.C) {
	if _, err := buildImage("dataimage",
		`FROM busybox
		 RUN mkdir -p /foo
		 RUN touch /foo/bar`,
		true); err != nil {
		c.Fatal(err)
	}

	cmd := exec.Command(dockerBinary, "run", "--name", "test", "-v", "/foo", "busybox")
	if _, err := runCommand(cmd); err != nil {
		c.Fatal(err)
	}

	cmd = exec.Command(dockerBinary, "run", "--volumes-from", "test", "dataimage", "ls", "-lh", "/foo/bar")
	if out, _, err := runCommandWithOutput(cmd); err == nil || !strings.Contains(out, "No such file or directory") {
		c.Fatalf("Data was copied on volumes-from but shouldn't be:\n%q", out)
	}

	tmpDir := randomUnixTmpDirPath("docker_test_bind_mount_copy_data")
	cmd = exec.Command(dockerBinary, "run", "-v", tmpDir+":/foo", "dataimage", "ls", "-lh", "/foo/bar")
	if out, _, err := runCommandWithOutput(cmd); err == nil || !strings.Contains(out, "No such file or directory") {
		c.Fatalf("Data was copied on bind-mount but shouldn't be:\n%q", out)
	}
}

func (s *DockerSuite) TestRunNoOutputFromPullInStdout(c *check.C) {
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

func (s *DockerSuite) TestRunVolumesCleanPaths(c *check.C) {
	if _, err := buildImage("run_volumes_clean_paths",
		`FROM busybox
		 VOLUME /foo/`,
		true); err != nil {
		c.Fatal(err)
	}

	cmd := exec.Command(dockerBinary, "run", "-v", "/foo", "-v", "/bar/", "--name", "dark_helmet", "run_volumes_clean_paths")
	if out, _, err := runCommandWithOutput(cmd); err != nil {
		c.Fatal(err, out)
	}

	out, err := inspectFieldMap("dark_helmet", "Volumes", "/foo/")
	c.Assert(err, check.IsNil)
	if out != "" {
		c.Fatalf("Found unexpected volume entry for '/foo/' in volumes\n%q", out)
	}

	out, err = inspectFieldMap("dark_helmet", "Volumes", "/foo")
	c.Assert(err, check.IsNil)
	if !strings.Contains(out, volumesConfigPath) {
		c.Fatalf("Volume was not defined for /foo\n%q", out)
	}

	out, err = inspectFieldMap("dark_helmet", "Volumes", "/bar/")
	c.Assert(err, check.IsNil)
	if out != "" {
		c.Fatalf("Found unexpected volume entry for '/bar/' in volumes\n%q", out)
	}
	out, err = inspectFieldMap("dark_helmet", "Volumes", "/bar")
	c.Assert(err, check.IsNil)
	if !strings.Contains(out, volumesConfigPath) {
		c.Fatalf("Volume was not defined for /bar\n%q", out)
	}
}

// Regression test for #3631
func (s *DockerSuite) TestRunSlowStdoutConsumer(c *check.C) {
	cont := exec.Command(dockerBinary, "run", "--rm", "busybox", "/bin/sh", "-c", "dd if=/dev/zero of=/dev/stdout bs=1024 count=2000 | catv")

	stdout, err := cont.StdoutPipe()
	if err != nil {
		c.Fatal(err)
	}

	if err := cont.Start(); err != nil {
		c.Fatal(err)
	}
	n, err := consumeWithSpeed(stdout, 10000, 5*time.Millisecond, nil)
	if err != nil {
		c.Fatal(err)
	}

	expected := 2 * 1024 * 2000
	if n != expected {
		c.Fatalf("Expected %d, got %d", expected, n)
	}
}

func (s *DockerSuite) TestRunAllowPortRangeThroughExpose(c *check.C) {
	cmd := exec.Command(dockerBinary, "run", "-d", "--expose", "3000-3003", "-P", "busybox", "top")
	out, _, err := runCommandWithOutput(cmd)
	if err != nil {
		c.Fatal(err)
	}
	id := strings.TrimSpace(out)
	portstr, err := inspectFieldJSON(id, "NetworkSettings.Ports")
	c.Assert(err, check.IsNil)
	var ports nat.PortMap
	if err = unmarshalJSON([]byte(portstr), &ports); err != nil {
		c.Fatal(err)
	}
	for port, binding := range ports {
		portnum, _ := strconv.Atoi(strings.Split(string(port), "/")[0])
		if portnum < 3000 || portnum > 3003 {
			c.Fatalf("Port %d is out of range ", portnum)
		}
		if binding == nil || len(binding) != 1 || len(binding[0].HostPort) == 0 {
			c.Fatalf("Port is not mapped for the port %d", port)
		}
	}
}

// test docker run expose a invalid port
func (s *DockerSuite) TestRunExposePort(c *check.C) {
	runCmd := exec.Command(dockerBinary, "run", "--expose", "80000", "busybox")
	out, _, err := runCommandWithOutput(runCmd)
	//expose a invalid port should with a error out
	if err == nil || !strings.Contains(out, "Invalid range format for --expose") {
		c.Fatalf("run --expose a invalid port should with error out")
	}
}

func (s *DockerSuite) TestRunUnknownCommand(c *check.C) {
	testRequires(c, NativeExecDriver)
	runCmd := exec.Command(dockerBinary, "create", "busybox", "/bin/nada")
	cID, _, _, err := runCommandWithStdoutStderr(runCmd)
	if err != nil {
		c.Fatalf("Failed to create container: %v, output: %q", err, cID)
	}
	cID = strings.TrimSpace(cID)

	runCmd = exec.Command(dockerBinary, "start", cID)
	_, _, _, _ = runCommandWithStdoutStderr(runCmd)

	rc, err := inspectField(cID, "State.ExitCode")
	c.Assert(err, check.IsNil)
	if rc == "0" {
		c.Fatalf("ExitCode(%v) cannot be 0", rc)
	}
}

func (s *DockerSuite) TestRunModeIpcHost(c *check.C) {
	testRequires(c, SameHostDaemon)

	hostIpc, err := os.Readlink("/proc/1/ns/ipc")
	if err != nil {
		c.Fatal(err)
	}

	cmd := exec.Command(dockerBinary, "run", "--ipc=host", "busybox", "readlink", "/proc/self/ns/ipc")
	out2, _, err := runCommandWithOutput(cmd)
	if err != nil {
		c.Fatal(err, out2)
	}

	out2 = strings.Trim(out2, "\n")
	if hostIpc != out2 {
		c.Fatalf("IPC different with --ipc=host %s != %s\n", hostIpc, out2)
	}

	cmd = exec.Command(dockerBinary, "run", "busybox", "readlink", "/proc/self/ns/ipc")
	out2, _, err = runCommandWithOutput(cmd)
	if err != nil {
		c.Fatal(err, out2)
	}

	out2 = strings.Trim(out2, "\n")
	if hostIpc == out2 {
		c.Fatalf("IPC should be different without --ipc=host %s == %s\n", hostIpc, out2)
	}
}

func (s *DockerSuite) TestRunModeIpcContainer(c *check.C) {
	testRequires(c, SameHostDaemon)

	cmd := exec.Command(dockerBinary, "run", "-d", "busybox", "top")
	out, _, err := runCommandWithOutput(cmd)
	if err != nil {
		c.Fatal(err, out)
	}
	id := strings.TrimSpace(out)
	state, err := inspectField(id, "State.Running")
	c.Assert(err, check.IsNil)
	if state != "true" {
		c.Fatal("Container state is 'not running'")
	}
	pid1, err := inspectField(id, "State.Pid")
	c.Assert(err, check.IsNil)

	parentContainerIpc, err := os.Readlink(fmt.Sprintf("/proc/%s/ns/ipc", pid1))
	if err != nil {
		c.Fatal(err)
	}
	cmd = exec.Command(dockerBinary, "run", fmt.Sprintf("--ipc=container:%s", id), "busybox", "readlink", "/proc/self/ns/ipc")
	out2, _, err := runCommandWithOutput(cmd)
	if err != nil {
		c.Fatal(err, out2)
	}

	out2 = strings.Trim(out2, "\n")
	if parentContainerIpc != out2 {
		c.Fatalf("IPC different with --ipc=container:%s %s != %s\n", id, parentContainerIpc, out2)
	}
}

func (s *DockerSuite) TestRunModeIpcContainerNotExists(c *check.C) {
	cmd := exec.Command(dockerBinary, "run", "-d", "--ipc", "container:abcd1234", "busybox", "top")
	out, _, err := runCommandWithOutput(cmd)
	if !strings.Contains(out, "abcd1234") || err == nil {
		c.Fatalf("run IPC from a non exists container should with correct error out")
	}
}

func (s *DockerSuite) TestRunModeIpcContainerNotRunning(c *check.C) {
	testRequires(c, SameHostDaemon)

	cmd := exec.Command(dockerBinary, "create", "busybox")
	out, _, err := runCommandWithOutput(cmd)
	if err != nil {
		c.Fatal(err, out)
	}
	id := strings.TrimSpace(out)

	cmd = exec.Command(dockerBinary, "run", fmt.Sprintf("--ipc=container:%s", id), "busybox")
	out, _, err = runCommandWithOutput(cmd)
	if err == nil {
		c.Fatalf("Run container with ipc mode container should fail with non running container: %s\n%s", out, err)
	}
}

func (s *DockerSuite) TestContainerNetworkMode(c *check.C) {
	testRequires(c, SameHostDaemon)

	cmd := exec.Command(dockerBinary, "run", "-d", "busybox", "top")
	out, _, err := runCommandWithOutput(cmd)
	if err != nil {
		c.Fatal(err, out)
	}
	id := strings.TrimSpace(out)
	if err := waitRun(id); err != nil {
		c.Fatal(err)
	}
	pid1, err := inspectField(id, "State.Pid")
	c.Assert(err, check.IsNil)

	parentContainerNet, err := os.Readlink(fmt.Sprintf("/proc/%s/ns/net", pid1))
	if err != nil {
		c.Fatal(err)
	}
	cmd = exec.Command(dockerBinary, "run", fmt.Sprintf("--net=container:%s", id), "busybox", "readlink", "/proc/self/ns/net")
	out2, _, err := runCommandWithOutput(cmd)
	if err != nil {
		c.Fatal(err, out2)
	}

	out2 = strings.Trim(out2, "\n")
	if parentContainerNet != out2 {
		c.Fatalf("NET different with --net=container:%s %s != %s\n", id, parentContainerNet, out2)
	}
}

func (s *DockerSuite) TestContainerNetworkModeToSelf(c *check.C) {
	cmd := exec.Command(dockerBinary, "run", "--name=me", "--net=container:me", "busybox", "true")
	out, _, err := runCommandWithOutput(cmd)
	if err == nil || !strings.Contains(out, "cannot join own network") {
		c.Fatalf("using container net mode to self should result in an error")
	}
}

func (s *DockerSuite) TestRunModePidHost(c *check.C) {
	testRequires(c, NativeExecDriver, SameHostDaemon)

	hostPid, err := os.Readlink("/proc/1/ns/pid")
	if err != nil {
		c.Fatal(err)
	}

	cmd := exec.Command(dockerBinary, "run", "--pid=host", "busybox", "readlink", "/proc/self/ns/pid")
	out2, _, err := runCommandWithOutput(cmd)
	if err != nil {
		c.Fatal(err, out2)
	}

	out2 = strings.Trim(out2, "\n")
	if hostPid != out2 {
		c.Fatalf("PID different with --pid=host %s != %s\n", hostPid, out2)
	}

	cmd = exec.Command(dockerBinary, "run", "busybox", "readlink", "/proc/self/ns/pid")
	out2, _, err = runCommandWithOutput(cmd)
	if err != nil {
		c.Fatal(err, out2)
	}

	out2 = strings.Trim(out2, "\n")
	if hostPid == out2 {
		c.Fatalf("PID should be different without --pid=host %s == %s\n", hostPid, out2)
	}
}

func (s *DockerSuite) TestRunModeUTSHost(c *check.C) {
	testRequires(c, NativeExecDriver, SameHostDaemon)

	hostUTS, err := os.Readlink("/proc/1/ns/uts")
	if err != nil {
		c.Fatal(err)
	}

	cmd := exec.Command(dockerBinary, "run", "--uts=host", "busybox", "readlink", "/proc/self/ns/uts")
	out2, _, err := runCommandWithOutput(cmd)
	if err != nil {
		c.Fatal(err, out2)
	}

	out2 = strings.Trim(out2, "\n")
	if hostUTS != out2 {
		c.Fatalf("UTS different with --uts=host %s != %s\n", hostUTS, out2)
	}

	cmd = exec.Command(dockerBinary, "run", "busybox", "readlink", "/proc/self/ns/uts")
	out2, _, err = runCommandWithOutput(cmd)
	if err != nil {
		c.Fatal(err, out2)
	}

	out2 = strings.Trim(out2, "\n")
	if hostUTS == out2 {
		c.Fatalf("UTS should be different without --uts=host %s == %s\n", hostUTS, out2)
	}
}

func (s *DockerSuite) TestRunTLSverify(c *check.C) {
	cmd := exec.Command(dockerBinary, "ps")
	out, ec, err := runCommandWithOutput(cmd)
	if err != nil || ec != 0 {
		c.Fatalf("Should have worked: %v:\n%v", err, out)
	}

	// Regardless of whether we specify true or false we need to
	// test to make sure tls is turned on if --tlsverify is specified at all

	cmd = exec.Command(dockerBinary, "--tlsverify=false", "ps")
	out, ec, err = runCommandWithOutput(cmd)
	if err == nil || ec == 0 || !strings.Contains(out, "trying to connect") {
		c.Fatalf("Should have failed: \net:%v\nout:%v\nerr:%v", ec, out, err)
	}

	cmd = exec.Command(dockerBinary, "--tlsverify=true", "ps")
	out, ec, err = runCommandWithOutput(cmd)
	if err == nil || ec == 0 || !strings.Contains(out, "cert") {
		c.Fatalf("Should have failed: \net:%v\nout:%v\nerr:%v", ec, out, err)
	}
}

func (s *DockerSuite) TestRunPortFromDockerRangeInUse(c *check.C) {
	// first find allocator current position
	cmd := exec.Command(dockerBinary, "run", "-d", "-p", ":80", "busybox", "top")
	out, _, err := runCommandWithOutput(cmd)
	if err != nil {
		c.Fatal(out, err)
	}
	id := strings.TrimSpace(out)
	cmd = exec.Command(dockerBinary, "port", id)
	out, _, err = runCommandWithOutput(cmd)
	if err != nil {
		c.Fatal(out, err)
	}
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
	cmd = exec.Command(dockerBinary, "run", "-d", "-p", ":80", "busybox", "top")
	out, _, err = runCommandWithOutput(cmd)
	if err != nil {
		c.Fatalf(out, err)
	}
	id = strings.TrimSpace(out)
	cmd = exec.Command(dockerBinary, "port", id)
	out, _, err = runCommandWithOutput(cmd)
	if err != nil {
		c.Fatal(out, err)
	}
}

func (s *DockerSuite) TestRunTtyWithPipe(c *check.C) {
	errChan := make(chan error)
	go func() {
		defer close(errChan)

		cmd := exec.Command(dockerBinary, "run", "-ti", "busybox", "true")
		if _, err := cmd.StdinPipe(); err != nil {
			errChan <- err
			return
		}

		expected := "cannot enable tty mode"
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
		c.Assert(err, check.IsNil)
	case <-time.After(3 * time.Second):
		c.Fatal("container is running but should have failed")
	}
}

func (s *DockerSuite) TestRunNonLocalMacAddress(c *check.C) {
	addr := "00:16:3E:08:00:50"

	cmd := exec.Command(dockerBinary, "run", "--mac-address", addr, "busybox", "ifconfig")
	if out, _, err := runCommandWithOutput(cmd); err != nil || !strings.Contains(out, addr) {
		c.Fatalf("Output should have contained %q: %s, %v", addr, out, err)
	}
}

func (s *DockerSuite) TestRunNetHost(c *check.C) {
	testRequires(c, SameHostDaemon)

	hostNet, err := os.Readlink("/proc/1/ns/net")
	if err != nil {
		c.Fatal(err)
	}

	cmd := exec.Command(dockerBinary, "run", "--net=host", "busybox", "readlink", "/proc/self/ns/net")
	out2, _, err := runCommandWithOutput(cmd)
	if err != nil {
		c.Fatal(err, out2)
	}

	out2 = strings.Trim(out2, "\n")
	if hostNet != out2 {
		c.Fatalf("Net namespace different with --net=host %s != %s\n", hostNet, out2)
	}

	cmd = exec.Command(dockerBinary, "run", "busybox", "readlink", "/proc/self/ns/net")
	out2, _, err = runCommandWithOutput(cmd)
	if err != nil {
		c.Fatal(err, out2)
	}

	out2 = strings.Trim(out2, "\n")
	if hostNet == out2 {
		c.Fatalf("Net namespace should be different without --net=host %s == %s\n", hostNet, out2)
	}
}

func (s *DockerSuite) TestRunNetContainerWhichHost(c *check.C) {
	testRequires(c, SameHostDaemon)

	hostNet, err := os.Readlink("/proc/1/ns/net")
	if err != nil {
		c.Fatal(err)
	}

	cmd := exec.Command(dockerBinary, "run", "-d", "--net=host", "--name=test", "busybox", "top")
	out, _, err := runCommandWithOutput(cmd)
	if err != nil {
		c.Fatal(err, out)
	}

	cmd = exec.Command(dockerBinary, "run", "--net=container:test", "busybox", "readlink", "/proc/self/ns/net")
	out, _, err = runCommandWithOutput(cmd)
	if err != nil {
		c.Fatal(err, out)
	}

	out = strings.Trim(out, "\n")
	if hostNet != out {
		c.Fatalf("Container should have host network namespace")
	}
}

func (s *DockerSuite) TestRunAllowPortRangeThroughPublish(c *check.C) {
	cmd := exec.Command(dockerBinary, "run", "-d", "--expose", "3000-3003", "-p", "3000-3003", "busybox", "top")
	out, _, err := runCommandWithOutput(cmd)

	id := strings.TrimSpace(out)
	portstr, err := inspectFieldJSON(id, "NetworkSettings.Ports")
	c.Assert(err, check.IsNil)
	var ports nat.PortMap
	err = unmarshalJSON([]byte(portstr), &ports)
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

func (s *DockerSuite) TestRunOOMExitCode(c *check.C) {
	errChan := make(chan error)
	go func() {
		defer close(errChan)
		runCmd := exec.Command(dockerBinary, "run", "-m", "4MB", "busybox", "sh", "-c", "x=a; while true; do x=$x$x$x$x; done")
		out, exitCode, _ := runCommandWithOutput(runCmd)
		if expected := 137; exitCode != expected {
			errChan <- fmt.Errorf("wrong exit code for OOM container: expected %d, got %d (output: %q)", expected, exitCode, out)
		}
	}()

	select {
	case err := <-errChan:
		c.Assert(err, check.IsNil)
	case <-time.After(30 * time.Second):
		c.Fatal("Timeout waiting for container to die on OOM")
	}
}

func (s *DockerSuite) TestRunSetDefaultRestartPolicy(c *check.C) {
	runCmd := exec.Command(dockerBinary, "run", "-d", "--name", "test", "busybox", "top")
	if out, _, err := runCommandWithOutput(runCmd); err != nil {
		c.Fatalf("failed to run container: %v, output: %q", err, out)
	}
	out, err := inspectField("test", "HostConfig.RestartPolicy.Name")
	c.Assert(err, check.IsNil)
	if out != "no" {
		c.Fatalf("Set default restart policy failed")
	}
}

func (s *DockerSuite) TestRunRestartMaxRetries(c *check.C) {
	out, err := exec.Command(dockerBinary, "run", "-d", "--restart=on-failure:3", "busybox", "false").CombinedOutput()
	if err != nil {
		c.Fatal(string(out), err)
	}
	id := strings.TrimSpace(string(out))
	if err := waitInspect(id, "{{ .State.Restarting }} {{ .State.Running }}", "false false", 10); err != nil {
		c.Fatal(err)
	}
	count, err := inspectField(id, "RestartCount")
	c.Assert(err, check.IsNil)
	if count != "3" {
		c.Fatalf("Container was restarted %s times, expected %d", count, 3)
	}
	MaximumRetryCount, err := inspectField(id, "HostConfig.RestartPolicy.MaximumRetryCount")
	c.Assert(err, check.IsNil)
	if MaximumRetryCount != "3" {
		c.Fatalf("Container Maximum Retry Count is %s, expected %s", MaximumRetryCount, "3")
	}
}

func (s *DockerSuite) TestRunContainerWithWritableRootfs(c *check.C) {
	out, err := exec.Command(dockerBinary, "run", "--rm", "busybox", "touch", "/file").CombinedOutput()
	if err != nil {
		c.Fatal(string(out), err)
	}
}

func (s *DockerSuite) TestRunContainerWithReadonlyRootfs(c *check.C) {
	testRequires(c, NativeExecDriver)

	for _, f := range []string{"/file", "/etc/hosts", "/etc/resolv.conf", "/etc/hostname"} {
		testReadOnlyFile(f, c)
	}
}

func testReadOnlyFile(filename string, c *check.C) {
	testRequires(c, NativeExecDriver)

	out, err := exec.Command(dockerBinary, "run", "--read-only", "--rm", "busybox", "touch", filename).CombinedOutput()
	if err == nil {
		c.Fatal("expected container to error on run with read only error")
	}
	expected := "Read-only file system"
	if !strings.Contains(string(out), expected) {
		c.Fatalf("expected output from failure to contain %s but contains %s", expected, out)
	}
}

func (s *DockerSuite) TestRunContainerWithReadonlyEtcHostsAndLinkedContainer(c *check.C) {
	testRequires(c, NativeExecDriver)

	_, err := runCommand(exec.Command(dockerBinary, "run", "-d", "--name", "test-etc-hosts-ro-linked", "busybox", "top"))
	c.Assert(err, check.IsNil)

	out, _, err := runCommandWithOutput(exec.Command(dockerBinary, "run", "--read-only", "--link", "test-etc-hosts-ro-linked:testlinked", "busybox", "cat", "/etc/hosts"))
	c.Assert(err, check.IsNil)

	if !strings.Contains(string(out), "testlinked") {
		c.Fatal("Expected /etc/hosts to be updated even if --read-only enabled")
	}
}

func (s *DockerSuite) TestRunContainerWithReadonlyRootfsWithDnsFlag(c *check.C) {
	testRequires(c, NativeExecDriver)

	out, _, err := runCommandWithOutput(exec.Command(dockerBinary, "run", "--read-only", "--dns", "1.1.1.1", "busybox", "/bin/cat", "/etc/resolv.conf"))
	c.Assert(err, check.IsNil)

	if !strings.Contains(string(out), "1.1.1.1") {
		c.Fatal("Expected /etc/resolv.conf to be updated even if --read-only enabled and --dns flag used")
	}
}

func (s *DockerSuite) TestRunContainerWithReadonlyRootfsWithAddHostFlag(c *check.C) {
	testRequires(c, NativeExecDriver)

	out, _, err := runCommandWithOutput(exec.Command(dockerBinary, "run", "--read-only", "--add-host", "testreadonly:127.0.0.1", "busybox", "/bin/cat", "/etc/hosts"))
	c.Assert(err, check.IsNil)

	if !strings.Contains(string(out), "testreadonly") {
		c.Fatal("Expected /etc/hosts to be updated even if --read-only enabled and --add-host flag used")
	}
}

func (s *DockerSuite) TestRunVolumesFromRestartAfterRemoved(c *check.C) {
	out, _, err := runCommandWithOutput(exec.Command(dockerBinary, "run", "-d", "--name", "voltest", "-v", "/foo", "busybox"))
	if err != nil {
		c.Fatal(out, err)
	}

	out, _, err = runCommandWithOutput(exec.Command(dockerBinary, "run", "-d", "--name", "restarter", "--volumes-from", "voltest", "busybox", "top"))
	if err != nil {
		c.Fatal(out, err)
	}

	// Remove the main volume container and restart the consuming container
	out, _, err = runCommandWithOutput(exec.Command(dockerBinary, "rm", "-f", "voltest"))
	if err != nil {
		c.Fatal(out, err)
	}

	// This should not fail since the volumes-from were already applied
	out, _, err = runCommandWithOutput(exec.Command(dockerBinary, "restart", "restarter"))
	if err != nil {
		c.Fatalf("expected container to restart successfully: %v\n%s", err, out)
	}
}

// run container with --rm should remove container if exit code != 0
func (s *DockerSuite) TestRunContainerWithRmFlagExitCodeNotEqualToZero(c *check.C) {
	name := "flowers"
	runCmd := exec.Command(dockerBinary, "run", "--name", name, "--rm", "busybox", "ls", "/notexists")
	out, _, err := runCommandWithOutput(runCmd)
	if err == nil {
		c.Fatal("Expected docker run to fail", out, err)
	}

	out, err = getAllContainers()
	if err != nil {
		c.Fatal(out, err)
	}

	if out != "" {
		c.Fatal("Expected not to have containers", out)
	}
}

func (s *DockerSuite) TestRunContainerWithRmFlagCannotStartContainer(c *check.C) {
	name := "sparkles"
	runCmd := exec.Command(dockerBinary, "run", "--name", name, "--rm", "busybox", "commandNotFound")
	out, _, err := runCommandWithOutput(runCmd)
	if err == nil {
		c.Fatal("Expected docker run to fail", out, err)
	}

	out, err = getAllContainers()
	if err != nil {
		c.Fatal(out, err)
	}

	if out != "" {
		c.Fatal("Expected not to have containers", out)
	}
}

func (s *DockerSuite) TestRunPidHostWithChildIsKillable(c *check.C) {
	name := "ibuildthecloud"
	if out, err := exec.Command(dockerBinary, "run", "-d", "--pid=host", "--name", name, "busybox", "sh", "-c", "sleep 30; echo hi").CombinedOutput(); err != nil {
		c.Fatal(err, out)
	}
	time.Sleep(1 * time.Second)
	errchan := make(chan error)
	go func() {
		if out, err := exec.Command(dockerBinary, "kill", name).CombinedOutput(); err != nil {
			errchan <- fmt.Errorf("%v:\n%s", err, out)
		}
		close(errchan)
	}()
	select {
	case err := <-errchan:
		c.Assert(err, check.IsNil)
	case <-time.After(5 * time.Second):
		c.Fatal("Kill container timed out")
	}
}

func (s *DockerSuite) TestRunWithTooSmallMemoryLimit(c *check.C) {
	// this memory limit is 1 byte less than the min, which is 4MB
	// https://github.com/docker/docker/blob/v1.5.0/daemon/create.go#L22
	out, _, err := runCommandWithOutput(exec.Command(dockerBinary, "run", "-m", "4194303", "busybox"))
	if err == nil || !strings.Contains(out, "Minimum memory limit allowed is 4MB") {
		c.Fatalf("expected run to fail when using too low a memory limit: %q", out)
	}
}

func (s *DockerSuite) TestRunWriteToProcAsound(c *check.C) {
	code, err := runCommand(exec.Command(dockerBinary, "run", "busybox", "sh", "-c", "echo 111 >> /proc/asound/version"))
	if err == nil || code == 0 {
		c.Fatal("standard container should not be able to write to /proc/asound")
	}
}

func (s *DockerSuite) TestRunReadProcTimer(c *check.C) {
	testRequires(c, NativeExecDriver)
	out, code, err := runCommandWithOutput(exec.Command(dockerBinary, "run", "busybox", "cat", "/proc/timer_stats"))
	if err != nil || code != 0 {
		c.Fatal(err)
	}
	if strings.Trim(out, "\n ") != "" {
		c.Fatalf("expected to receive no output from /proc/timer_stats but received %q", out)
	}
}

func (s *DockerSuite) TestRunReadProcLatency(c *check.C) {
	testRequires(c, NativeExecDriver)
	// some kernels don't have this configured so skip the test if this file is not found
	// on the host running the tests.
	if _, err := os.Stat("/proc/latency_stats"); err != nil {
		c.Skip("kernel doesnt have latency_stats configured")
		return
	}
	out, code, err := runCommandWithOutput(exec.Command(dockerBinary, "run", "busybox", "cat", "/proc/latency_stats"))
	if err != nil || code != 0 {
		c.Fatal(err)
	}
	if strings.Trim(out, "\n ") != "" {
		c.Fatalf("expected to receive no output from /proc/latency_stats but received %q", out)
	}
}

func (s *DockerSuite) TestMountIntoProc(c *check.C) {
	testRequires(c, NativeExecDriver)
	code, err := runCommand(exec.Command(dockerBinary, "run", "-v", "/proc//sys", "busybox", "true"))
	if err == nil || code == 0 {
		c.Fatal("container should not be able to mount into /proc")
	}
}

func (s *DockerSuite) TestMountIntoSys(c *check.C) {
	testRequires(c, NativeExecDriver)
	_, err := runCommand(exec.Command(dockerBinary, "run", "-v", "/sys/fs/cgroup", "busybox", "true"))
	if err != nil {
		c.Fatal("container should be able to mount into /sys/fs/cgroup")
	}
}

func (s *DockerSuite) TestTwoContainersInNetHost(c *check.C) {
	dockerCmd(c, "run", "-d", "--net=host", "--name=first", "busybox", "top")
	dockerCmd(c, "run", "-d", "--net=host", "--name=second", "busybox", "top")
	dockerCmd(c, "stop", "first")
	dockerCmd(c, "stop", "second")
}

func (s *DockerSuite) TestRunUnshareProc(c *check.C) {
	testRequires(c, Apparmor, NativeExecDriver)

	name := "acidburn"
	runCmd := exec.Command(dockerBinary, "run", "--name", name, "jess/unshare", "unshare", "-p", "-m", "-f", "-r", "--mount-proc=/proc", "mount")
	if out, _, err := runCommandWithOutput(runCmd); err == nil || !strings.Contains(out, "Permission denied") {
		c.Fatalf("unshare should have failed with permission denied, got: %s, %v", out, err)
	}

	name = "cereal"
	runCmd = exec.Command(dockerBinary, "run", "--name", name, "jess/unshare", "unshare", "-p", "-m", "-f", "-r", "mount", "-t", "proc", "none", "/proc")
	if out, _, err := runCommandWithOutput(runCmd); err == nil || !strings.Contains(out, "Permission denied") {
		c.Fatalf("unshare should have failed with permission denied, got: %s, %v", out, err)
	}

	/* Ensure still fails if running privileged with the default policy */
	name = "crashoverride"
	runCmd = exec.Command(dockerBinary, "run", "--privileged", "--security-opt", "apparmor:docker-default", "--name", name, "jess/unshare", "unshare", "-p", "-m", "-f", "-r", "mount", "-t", "proc", "none", "/proc")
	if out, _, err := runCommandWithOutput(runCmd); err == nil || !strings.Contains(out, "Permission denied") {
		c.Fatalf("unshare should have failed with permission denied, got: %s, %v", out, err)
	}
}

func (s *DockerSuite) TestRunPublishPort(c *check.C) {
	out, _, err := runCommandWithOutput(exec.Command(dockerBinary, "run", "-d", "--name", "test", "--expose", "8080", "busybox", "top"))
	c.Assert(err, check.IsNil)
	out, _, err = runCommandWithOutput(exec.Command(dockerBinary, "port", "test"))
	c.Assert(err, check.IsNil)
	out = strings.Trim(out, "\r\n")
	if out != "" {
		c.Fatalf("run without --publish-all should not publish port, out should be nil, but got: %s", out)
	}
}

// Issue #10184.
func (s *DockerSuite) TestDevicePermissions(c *check.C) {
	testRequires(c, NativeExecDriver)
	const permissions = "crw-rw-rw-"
	out, status := dockerCmd(c, "run", "--device", "/dev/fuse:/dev/fuse:mrw", "busybox:latest", "ls", "-l", "/dev/fuse")
	if status != 0 {
		c.Fatalf("expected status 0, got %d", status)
	}
	if !strings.HasPrefix(out, permissions) {
		c.Fatalf("output should begin with %q, got %q", permissions, out)
	}
}
