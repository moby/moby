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
	"testing"
	"time"

	"github.com/docker/docker/nat"
	"github.com/docker/docker/pkg/mount"
	"github.com/docker/docker/pkg/networkfs/resolvconf"
	"github.com/kr/pty"
)

// "test123" should be printed by docker run
func TestRunEchoStdout(t *testing.T) {
	runCmd := exec.Command(dockerBinary, "run", "busybox", "echo", "test123")
	out, _, _, err := runCommandWithStdoutStderr(runCmd)
	if err != nil {
		t.Fatalf("failed to run container: %v, output: %q", err, out)
	}

	if out != "test123\n" {
		t.Errorf("container should've printed 'test123'")
	}

	deleteAllContainers()

	logDone("run - echo test123")
}

// "test" should be printed
func TestRunEchoStdoutWithMemoryLimit(t *testing.T) {
	runCmd := exec.Command(dockerBinary, "run", "-m", "16m", "busybox", "echo", "test")
	out, _, _, err := runCommandWithStdoutStderr(runCmd)
	if err != nil {
		t.Fatalf("failed to run container: %v, output: %q", err, out)
	}

	out = strings.Trim(out, "\r\n")

	if expected := "test"; out != expected {
		t.Errorf("container should've printed %q but printed %q", expected, out)

	}

	deleteAllContainers()

	logDone("run - echo with memory limit")
}

// "test" should be printed
func TestRunEchoStdoutWitCPULimit(t *testing.T) {
	runCmd := exec.Command(dockerBinary, "run", "-c", "1000", "busybox", "echo", "test")
	out, _, _, err := runCommandWithStdoutStderr(runCmd)
	if err != nil {
		t.Fatalf("failed to run container: %v, output: %q", err, out)
	}

	if out != "test\n" {
		t.Errorf("container should've printed 'test'")
	}

	deleteAllContainers()

	logDone("run - echo with CPU limit")
}

// "test" should be printed
func TestRunEchoStdoutWithCPUAndMemoryLimit(t *testing.T) {
	runCmd := exec.Command(dockerBinary, "run", "-c", "1000", "-m", "16m", "busybox", "echo", "test")
	out, _, _, err := runCommandWithStdoutStderr(runCmd)
	if err != nil {
		t.Fatalf("failed to run container: %v, output: %q", err, out)
	}

	if out != "test\n" {
		t.Errorf("container should've printed 'test', got %q instead", out)
	}

	deleteAllContainers()

	logDone("run - echo with CPU and memory limit")
}

// "test" should be printed
func TestRunEchoNamedContainer(t *testing.T) {
	runCmd := exec.Command(dockerBinary, "run", "--name", "testfoonamedcontainer", "busybox", "echo", "test")
	out, _, _, err := runCommandWithStdoutStderr(runCmd)
	if err != nil {
		t.Fatalf("failed to run container: %v, output: %q", err, out)
	}

	if out != "test\n" {
		t.Errorf("container should've printed 'test'")
	}

	if err := deleteContainer("testfoonamedcontainer"); err != nil {
		t.Errorf("failed to remove the named container: %v", err)
	}

	deleteAllContainers()

	logDone("run - echo with named container")
}

// docker run should not leak file descriptors
func TestRunLeakyFileDescriptors(t *testing.T) {
	runCmd := exec.Command(dockerBinary, "run", "busybox", "ls", "-C", "/proc/self/fd")
	out, _, _, err := runCommandWithStdoutStderr(runCmd)
	if err != nil {
		t.Fatalf("failed to run container: %v, output: %q", err, out)
	}

	// normally, we should only get 0, 1, and 2, but 3 gets created by "ls" when it does "opendir" on the "fd" directory
	if out != "0  1  2  3\n" {
		t.Errorf("container should've printed '0  1  2  3', not: %s", out)
	}

	deleteAllContainers()

	logDone("run - check file descriptor leakage")
}

// it should be possible to ping Google DNS resolver
// this will fail when Internet access is unavailable
func TestRunPingGoogle(t *testing.T) {
	runCmd := exec.Command(dockerBinary, "run", "busybox", "ping", "-c", "1", "8.8.8.8")
	out, _, _, err := runCommandWithStdoutStderr(runCmd)
	if err != nil {
		t.Fatalf("failed to run container: %v, output: %q", err, out)
	}

	deleteAllContainers()

	logDone("run - ping 8.8.8.8")
}

// the exit code should be 0
// some versions of lxc might make this test fail
func TestRunExitCodeZero(t *testing.T) {
	runCmd := exec.Command(dockerBinary, "run", "busybox", "true")
	if out, _, err := runCommandWithOutput(runCmd); err != nil {
		t.Errorf("container should've exited with exit code 0: %s, %v", out, err)
	}

	deleteAllContainers()

	logDone("run - exit with 0")
}

// the exit code should be 1
// some versions of lxc might make this test fail
func TestRunExitCodeOne(t *testing.T) {
	runCmd := exec.Command(dockerBinary, "run", "busybox", "false")
	exitCode, err := runCommand(runCmd)
	if err != nil && !strings.Contains("exit status 1", fmt.Sprintf("%s", err)) {
		t.Fatal(err)
	}
	if exitCode != 1 {
		t.Errorf("container should've exited with exit code 1")
	}

	deleteAllContainers()

	logDone("run - exit with 1")
}

// it should be possible to pipe in data via stdin to a process running in a container
// some versions of lxc might make this test fail
func TestRunStdinPipe(t *testing.T) {
	runCmd := exec.Command("bash", "-c", `echo "blahblah" | docker run -i -a stdin busybox cat`)
	out, _, _, err := runCommandWithStdoutStderr(runCmd)
	if err != nil {
		t.Fatalf("failed to run container: %v, output: %q", err, out)
	}

	out = stripTrailingCharacters(out)

	inspectCmd := exec.Command(dockerBinary, "inspect", out)
	if out, _, err := runCommandWithOutput(inspectCmd); err != nil {
		t.Fatalf("out should've been a container id: %s %v", out, err)
	}

	waitCmd := exec.Command(dockerBinary, "wait", out)
	if waitOut, _, err := runCommandWithOutput(waitCmd); err != nil {
		t.Fatalf("error thrown while waiting for container: %s, %v", waitOut, err)
	}

	logsCmd := exec.Command(dockerBinary, "logs", out)
	logsOut, _, err := runCommandWithOutput(logsCmd)
	if err != nil {
		t.Fatalf("error thrown while trying to get container logs: %s, %v", logsOut, err)
	}

	containerLogs := stripTrailingCharacters(logsOut)

	if containerLogs != "blahblah" {
		t.Errorf("logs didn't print the container's logs %s", containerLogs)
	}

	rmCmd := exec.Command(dockerBinary, "rm", out)
	if out, _, err = runCommandWithOutput(rmCmd); err != nil {
		t.Fatalf("rm failed to remove container: %s, %v", out, err)
	}

	deleteAllContainers()

	logDone("run - pipe in with -i -a stdin")
}

// the container's ID should be printed when starting a container in detached mode
func TestRunDetachedContainerIDPrinting(t *testing.T) {
	runCmd := exec.Command(dockerBinary, "run", "-d", "busybox", "true")
	out, _, _, err := runCommandWithStdoutStderr(runCmd)
	if err != nil {
		t.Fatalf("failed to run container: %v, output: %q", err, out)
	}

	out = stripTrailingCharacters(out)

	inspectCmd := exec.Command(dockerBinary, "inspect", out)
	if inspectOut, _, err := runCommandWithOutput(inspectCmd); err != nil {
		t.Fatalf("out should've been a container id: %s %v", inspectOut, err)
	}

	waitCmd := exec.Command(dockerBinary, "wait", out)
	if waitOut, _, err := runCommandWithOutput(waitCmd); err != nil {
		t.Fatalf("error thrown while waiting for container: %s, %v", waitOut, err)
	}

	rmCmd := exec.Command(dockerBinary, "rm", out)
	rmOut, _, err := runCommandWithOutput(rmCmd)
	if err != nil {
		t.Fatalf("rm failed to remove container: %s, %v", rmOut, err)
	}

	rmOut = stripTrailingCharacters(rmOut)
	if rmOut != out {
		t.Errorf("rm didn't print the container ID %s %s", out, rmOut)
	}

	deleteAllContainers()

	logDone("run - print container ID in detached mode")
}

// the working directory should be set correctly
func TestRunWorkingDirectory(t *testing.T) {
	runCmd := exec.Command(dockerBinary, "run", "-w", "/root", "busybox", "pwd")
	out, _, _, err := runCommandWithStdoutStderr(runCmd)
	if err != nil {
		t.Fatalf("failed to run container: %v, output: %q", err, out)
	}

	out = stripTrailingCharacters(out)

	if out != "/root" {
		t.Errorf("-w failed to set working directory")
	}

	runCmd = exec.Command(dockerBinary, "run", "--workdir", "/root", "busybox", "pwd")
	out, _, _, err = runCommandWithStdoutStderr(runCmd)
	if err != nil {
		t.Fatal(out, err)
	}

	out = stripTrailingCharacters(out)

	if out != "/root" {
		t.Errorf("--workdir failed to set working directory")
	}

	deleteAllContainers()

	logDone("run - run with working directory set by -w")
	logDone("run - run with working directory set by --workdir")
}

// pinging Google's DNS resolver should fail when we disable the networking
func TestRunWithoutNetworking(t *testing.T) {
	runCmd := exec.Command(dockerBinary, "run", "--net=none", "busybox", "ping", "-c", "1", "8.8.8.8")
	out, _, exitCode, err := runCommandWithStdoutStderr(runCmd)
	if err != nil && exitCode != 1 {
		t.Fatal(out, err)
	}
	if exitCode != 1 {
		t.Errorf("--net=none should've disabled the network; the container shouldn't have been able to ping 8.8.8.8")
	}

	runCmd = exec.Command(dockerBinary, "run", "-n=false", "busybox", "ping", "-c", "1", "8.8.8.8")
	out, _, exitCode, err = runCommandWithStdoutStderr(runCmd)
	if err != nil && exitCode != 1 {
		t.Fatal(out, err)
	}
	if exitCode != 1 {
		t.Errorf("-n=false should've disabled the network; the container shouldn't have been able to ping 8.8.8.8")
	}

	deleteAllContainers()

	logDone("run - disable networking with --net=none")
	logDone("run - disable networking with -n=false")
}

// Regression test for #4741
func TestRunWithVolumesAsFiles(t *testing.T) {
	runCmd := exec.Command(dockerBinary, "run", "--name", "test-data", "--volume", "/etc/hosts:/target-file", "busybox", "true")
	out, stderr, exitCode, err := runCommandWithStdoutStderr(runCmd)
	if err != nil && exitCode != 0 {
		t.Fatal("1", out, stderr, err)
	}

	runCmd = exec.Command(dockerBinary, "run", "--volumes-from", "test-data", "busybox", "cat", "/target-file")
	out, stderr, exitCode, err = runCommandWithStdoutStderr(runCmd)
	if err != nil && exitCode != 0 {
		t.Fatal("2", out, stderr, err)
	}
	deleteAllContainers()

	logDone("run - regression test for #4741 - volumes from as files")
}

// Regression test for #4979
func TestRunWithVolumesFromExited(t *testing.T) {
	runCmd := exec.Command(dockerBinary, "run", "--name", "test-data", "--volume", "/some/dir", "busybox", "touch", "/some/dir/file")
	out, stderr, exitCode, err := runCommandWithStdoutStderr(runCmd)
	if err != nil && exitCode != 0 {
		t.Fatal("1", out, stderr, err)
	}

	runCmd = exec.Command(dockerBinary, "run", "--volumes-from", "test-data", "busybox", "cat", "/some/dir/file")
	out, stderr, exitCode, err = runCommandWithStdoutStderr(runCmd)
	if err != nil && exitCode != 0 {
		t.Fatal("2", out, stderr, err)
	}
	deleteAllContainers()

	logDone("run - regression test for #4979 - volumes-from on exited container")
}

// Regression test for #4830
func TestRunWithRelativePath(t *testing.T) {
	runCmd := exec.Command(dockerBinary, "run", "-v", "tmp:/other-tmp", "busybox", "true")
	if _, _, _, err := runCommandWithStdoutStderr(runCmd); err == nil {
		t.Fatalf("relative path should result in an error")
	}

	deleteAllContainers()

	logDone("run - volume with relative path")
}

func TestRunVolumesMountedAsReadonly(t *testing.T) {
	cmd := exec.Command(dockerBinary, "run", "-v", "/test:/test:ro", "busybox", "touch", "/test/somefile")
	if code, err := runCommand(cmd); err == nil || code == 0 {
		t.Fatalf("run should fail because volume is ro: exit code %d", code)
	}

	deleteAllContainers()

	logDone("run - volumes as readonly mount")
}

func TestRunVolumesFromInReadonlyMode(t *testing.T) {
	cmd := exec.Command(dockerBinary, "run", "--name", "parent", "-v", "/test", "busybox", "true")
	if _, err := runCommand(cmd); err != nil {
		t.Fatal(err)
	}

	cmd = exec.Command(dockerBinary, "run", "--volumes-from", "parent:ro", "busybox", "touch", "/test/file")
	if code, err := runCommand(cmd); err == nil || code == 0 {
		t.Fatalf("run should fail because volume is ro: exit code %d", code)
	}

	deleteAllContainers()

	logDone("run - volumes from as readonly mount")
}

// Regression test for #1201
func TestRunVolumesFromInReadWriteMode(t *testing.T) {
	cmd := exec.Command(dockerBinary, "run", "--name", "parent", "-v", "/test", "busybox", "true")
	if _, err := runCommand(cmd); err != nil {
		t.Fatal(err)
	}

	cmd = exec.Command(dockerBinary, "run", "--volumes-from", "parent:rw", "busybox", "touch", "/test/file")
	if out, _, err := runCommandWithOutput(cmd); err != nil {
		t.Fatalf("running --volumes-from parent:rw failed with output: %q\nerror: %v", out, err)
	}

	cmd = exec.Command(dockerBinary, "run", "--volumes-from", "parent:bar", "busybox", "touch", "/test/file")
	if out, _, err := runCommandWithOutput(cmd); err == nil || !strings.Contains(out, "Invalid mode for volumes-from: bar") {
		t.Fatalf("running --volumes-from foo:bar should have failed with invalid mount mode: %q", out)
	}

	cmd = exec.Command(dockerBinary, "run", "--volumes-from", "parent", "busybox", "touch", "/test/file")
	if out, _, err := runCommandWithOutput(cmd); err != nil {
		t.Fatalf("running --volumes-from parent failed with output: %q\nerror: %v", out, err)
	}

	deleteAllContainers()

	logDone("run - volumes from as read write mount")
}

func TestVolumesFromGetsProperMode(t *testing.T) {
	cmd := exec.Command(dockerBinary, "run", "--name", "parent", "-v", "/test:/test:ro", "busybox", "true")
	if _, err := runCommand(cmd); err != nil {
		t.Fatal(err)
	}
	// Expect this "rw" mode to be be ignored since the inheritted volume is "ro"
	cmd = exec.Command(dockerBinary, "run", "--volumes-from", "parent:rw", "busybox", "touch", "/test/file")
	if _, err := runCommand(cmd); err == nil {
		t.Fatal("Expected volumes-from to inherit read-only volume even when passing in `rw`")
	}

	cmd = exec.Command(dockerBinary, "run", "--name", "parent2", "-v", "/test:/test:ro", "busybox", "true")
	if _, err := runCommand(cmd); err != nil {
		t.Fatal(err)
	}
	// Expect this to be read-only since both are "ro"
	cmd = exec.Command(dockerBinary, "run", "--volumes-from", "parent2:ro", "busybox", "touch", "/test/file")
	if _, err := runCommand(cmd); err == nil {
		t.Fatal("Expected volumes-from to inherit read-only volume even when passing in `ro`")
	}

	deleteAllContainers()

	logDone("run - volumes from ignores `rw` if inherrited volume is `ro`")
}

// Test for #1351
func TestRunApplyVolumesFromBeforeVolumes(t *testing.T) {
	cmd := exec.Command(dockerBinary, "run", "--name", "parent", "-v", "/test", "busybox", "touch", "/test/foo")
	if _, err := runCommand(cmd); err != nil {
		t.Fatal(err)
	}

	cmd = exec.Command(dockerBinary, "run", "--volumes-from", "parent", "-v", "/test", "busybox", "cat", "/test/foo")
	if out, _, err := runCommandWithOutput(cmd); err != nil {
		t.Fatal(out, err)
	}

	deleteAllContainers()

	logDone("run - volumes from mounted first")
}

func TestRunMultipleVolumesFrom(t *testing.T) {
	cmd := exec.Command(dockerBinary, "run", "--name", "parent1", "-v", "/test", "busybox", "touch", "/test/foo")
	if _, err := runCommand(cmd); err != nil {
		t.Fatal(err)
	}

	cmd = exec.Command(dockerBinary, "run", "--name", "parent2", "-v", "/other", "busybox", "touch", "/other/bar")
	if _, err := runCommand(cmd); err != nil {
		t.Fatal(err)
	}

	cmd = exec.Command(dockerBinary, "run", "--volumes-from", "parent1", "--volumes-from", "parent2",
		"busybox", "sh", "-c", "cat /test/foo && cat /other/bar")
	if _, err := runCommand(cmd); err != nil {
		t.Fatal(err)
	}

	deleteAllContainers()

	logDone("run - multiple volumes from")
}

// this tests verifies the ID format for the container
func TestRunVerifyContainerID(t *testing.T) {
	cmd := exec.Command(dockerBinary, "run", "-d", "busybox", "true")
	out, exit, err := runCommandWithOutput(cmd)
	if err != nil {
		t.Fatal(err)
	}
	if exit != 0 {
		t.Fatalf("expected exit code 0 received %d", exit)
	}
	match, err := regexp.MatchString("^[0-9a-f]{64}$", strings.TrimSuffix(out, "\n"))
	if err != nil {
		t.Fatal(err)
	}
	if !match {
		t.Fatalf("Invalid container ID: %s", out)
	}

	deleteAllContainers()

	logDone("run - verify container ID")
}

// Test that creating a container with a volume doesn't crash. Regression test for #995.
func TestRunCreateVolume(t *testing.T) {
	cmd := exec.Command(dockerBinary, "run", "-v", "/var/lib/data", "busybox", "true")
	if _, err := runCommand(cmd); err != nil {
		t.Fatal(err)
	}

	deleteAllContainers()

	logDone("run - create docker managed volume")
}

// Test that creating a volume with a symlink in its path works correctly. Test for #5152.
// Note that this bug happens only with symlinks with a target that starts with '/'.
func TestRunCreateVolumeWithSymlink(t *testing.T) {
	buildCmd := exec.Command(dockerBinary, "build", "-t", "docker-test-createvolumewithsymlink", "-")
	buildCmd.Stdin = strings.NewReader(`FROM busybox
		RUN mkdir /foo && ln -s /foo /bar`)
	buildCmd.Dir = workingDirectory
	err := buildCmd.Run()
	if err != nil {
		t.Fatalf("could not build 'docker-test-createvolumewithsymlink': %v", err)
	}

	cmd := exec.Command(dockerBinary, "run", "-v", "/bar/foo", "--name", "test-createvolumewithsymlink", "docker-test-createvolumewithsymlink", "sh", "-c", "mount | grep -q /foo/foo")
	exitCode, err := runCommand(cmd)
	if err != nil || exitCode != 0 {
		t.Fatalf("[run] err: %v, exitcode: %d", err, exitCode)
	}

	var volPath string
	cmd = exec.Command(dockerBinary, "inspect", "-f", "{{range .Volumes}}{{.}}{{end}}", "test-createvolumewithsymlink")
	volPath, exitCode, err = runCommandWithOutput(cmd)
	if err != nil || exitCode != 0 {
		t.Fatalf("[inspect] err: %v, exitcode: %d", err, exitCode)
	}

	cmd = exec.Command(dockerBinary, "rm", "-v", "test-createvolumewithsymlink")
	exitCode, err = runCommand(cmd)
	if err != nil || exitCode != 0 {
		t.Fatalf("[rm] err: %v, exitcode: %d", err, exitCode)
	}

	f, err := os.Open(volPath)
	defer f.Close()
	if !os.IsNotExist(err) {
		t.Fatalf("[open] (expecting 'file does not exist' error) err: %v, volPath: %s", err, volPath)
	}

	deleteImages("docker-test-createvolumewithsymlink")
	deleteAllContainers()

	logDone("run - create volume with symlink")
}

// Tests that a volume path that has a symlink exists in a container mounting it with `--volumes-from`.
func TestRunVolumesFromSymlinkPath(t *testing.T) {
	name := "docker-test-volumesfromsymlinkpath"
	buildCmd := exec.Command(dockerBinary, "build", "-t", name, "-")
	buildCmd.Stdin = strings.NewReader(`FROM busybox
		RUN mkdir /baz && ln -s /baz /foo
		VOLUME ["/foo/bar"]`)
	buildCmd.Dir = workingDirectory
	err := buildCmd.Run()
	if err != nil {
		t.Fatalf("could not build 'docker-test-volumesfromsymlinkpath': %v", err)
	}

	cmd := exec.Command(dockerBinary, "run", "--name", "test-volumesfromsymlinkpath", name)
	exitCode, err := runCommand(cmd)
	if err != nil || exitCode != 0 {
		t.Fatalf("[run] (volume) err: %v, exitcode: %d", err, exitCode)
	}

	cmd = exec.Command(dockerBinary, "run", "--volumes-from", "test-volumesfromsymlinkpath", "busybox", "sh", "-c", "ls /foo | grep -q bar")
	exitCode, err = runCommand(cmd)
	if err != nil || exitCode != 0 {
		t.Fatalf("[run] err: %v, exitcode: %d", err, exitCode)
	}

	deleteAllContainers()
	deleteImages(name)

	logDone("run - volumes-from symlink path")
}

func TestRunExitCode(t *testing.T) {
	cmd := exec.Command(dockerBinary, "run", "busybox", "/bin/sh", "-c", "exit 72")

	exit, err := runCommand(cmd)
	if err == nil {
		t.Fatal("should not have a non nil error")
	}
	if exit != 72 {
		t.Fatalf("expected exit code 72 received %d", exit)
	}

	deleteAllContainers()

	logDone("run - correct exit code")
}

func TestRunUserDefaultsToRoot(t *testing.T) {
	cmd := exec.Command(dockerBinary, "run", "busybox", "id")

	out, _, err := runCommandWithOutput(cmd)
	if err != nil {
		t.Fatal(err, out)
	}
	if !strings.Contains(out, "uid=0(root) gid=0(root)") {
		t.Fatalf("expected root user got %s", out)
	}
	deleteAllContainers()

	logDone("run - default user")
}

func TestRunUserByName(t *testing.T) {
	cmd := exec.Command(dockerBinary, "run", "-u", "root", "busybox", "id")

	out, _, err := runCommandWithOutput(cmd)
	if err != nil {
		t.Fatal(err, out)
	}
	if !strings.Contains(out, "uid=0(root) gid=0(root)") {
		t.Fatalf("expected root user got %s", out)
	}
	deleteAllContainers()

	logDone("run - user by name")
}

func TestRunUserByID(t *testing.T) {
	cmd := exec.Command(dockerBinary, "run", "-u", "1", "busybox", "id")

	out, _, err := runCommandWithOutput(cmd)
	if err != nil {
		t.Fatal(err, out)
	}
	if !strings.Contains(out, "uid=1(daemon) gid=1(daemon)") {
		t.Fatalf("expected daemon user got %s", out)
	}
	deleteAllContainers()

	logDone("run - user by id")
}

func TestRunUserByIDBig(t *testing.T) {
	cmd := exec.Command(dockerBinary, "run", "-u", "2147483648", "busybox", "id")

	out, _, err := runCommandWithOutput(cmd)
	if err == nil {
		t.Fatal("No error, but must be.", out)
	}
	if !strings.Contains(out, "Uids and gids must be in range") {
		t.Fatalf("expected error about uids range, got %s", out)
	}
	deleteAllContainers()

	logDone("run - user by id, id too big")
}

func TestRunUserByIDNegative(t *testing.T) {
	cmd := exec.Command(dockerBinary, "run", "-u", "-1", "busybox", "id")

	out, _, err := runCommandWithOutput(cmd)
	if err == nil {
		t.Fatal("No error, but must be.", out)
	}
	if !strings.Contains(out, "Uids and gids must be in range") {
		t.Fatalf("expected error about uids range, got %s", out)
	}
	deleteAllContainers()

	logDone("run - user by id, id negative")
}

func TestRunUserByIDZero(t *testing.T) {
	cmd := exec.Command(dockerBinary, "run", "-u", "0", "busybox", "id")

	out, _, err := runCommandWithOutput(cmd)
	if err != nil {
		t.Fatal(err, out)
	}
	if !strings.Contains(out, "uid=0(root) gid=0(root) groups=10(wheel)") {
		t.Fatalf("expected daemon user got %s", out)
	}
	deleteAllContainers()

	logDone("run - user by id, zero uid")
}

func TestRunUserNotFound(t *testing.T) {
	cmd := exec.Command(dockerBinary, "run", "-u", "notme", "busybox", "id")

	_, err := runCommand(cmd)
	if err == nil {
		t.Fatal("unknown user should cause container to fail")
	}
	deleteAllContainers()

	logDone("run - user not found")
}

func TestRunTwoConcurrentContainers(t *testing.T) {
	group := sync.WaitGroup{}
	group.Add(2)

	for i := 0; i < 2; i++ {
		go func() {
			defer group.Done()
			cmd := exec.Command(dockerBinary, "run", "busybox", "sleep", "2")
			if _, err := runCommand(cmd); err != nil {
				t.Fatal(err)
			}
		}()
	}

	group.Wait()

	deleteAllContainers()

	logDone("run - two concurrent containers")
}

func TestRunEnvironment(t *testing.T) {
	cmd := exec.Command(dockerBinary, "run", "-h", "testing", "-e=FALSE=true", "-e=TRUE", "-e=TRICKY", "-e=HOME=", "busybox", "env")
	cmd.Env = append(os.Environ(),
		"TRUE=false",
		"TRICKY=tri\ncky\n",
	)

	out, _, err := runCommandWithOutput(cmd)
	if err != nil {
		t.Fatal(err, out)
	}

	actualEnv := strings.Split(out, "\n")
	if actualEnv[len(actualEnv)-1] == "" {
		actualEnv = actualEnv[:len(actualEnv)-1]
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
		t.Fatalf("Wrong environment: should be %d variables, not: %q\n", len(goodEnv), strings.Join(actualEnv, ", "))
	}
	for i := range goodEnv {
		if actualEnv[i] != goodEnv[i] {
			t.Fatalf("Wrong environment variable: should be %s, not %s", goodEnv[i], actualEnv[i])
		}
	}

	deleteAllContainers()

	logDone("run - verify environment")
}

func TestRunContainerNetwork(t *testing.T) {
	cmd := exec.Command(dockerBinary, "run", "busybox", "ping", "-c", "1", "127.0.0.1")
	if _, err := runCommand(cmd); err != nil {
		t.Fatal(err)
	}

	deleteAllContainers()

	logDone("run - test container network via ping")
}

// Issue #4681
func TestRunLoopbackWhenNetworkDisabled(t *testing.T) {
	cmd := exec.Command(dockerBinary, "run", "--net=none", "busybox", "ping", "-c", "1", "127.0.0.1")
	if _, err := runCommand(cmd); err != nil {
		t.Fatal(err)
	}

	deleteAllContainers()

	logDone("run - test container loopback when networking disabled")
}

func TestRunNetHostNotAllowedWithLinks(t *testing.T) {
	_, _, err := dockerCmd(t, "run", "--name", "linked", "busybox", "true")

	cmd := exec.Command(dockerBinary, "run", "--net=host", "--link", "linked:linked", "busybox", "true")
	_, _, err = runCommandWithOutput(cmd)
	if err == nil {
		t.Fatal("Expected error")
	}

	deleteAllContainers()

	logDone("run - don't allow --net=host to be used with links")
}

func TestRunLoopbackOnlyExistsWhenNetworkingDisabled(t *testing.T) {
	cmd := exec.Command(dockerBinary, "run", "--net=none", "busybox", "ip", "-o", "-4", "a", "show", "up")
	out, _, err := runCommandWithOutput(cmd)
	if err != nil {
		t.Fatal(err, out)
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
		t.Fatalf("Wrong interface count in container %d", count)
	}

	if !strings.HasPrefix(out, "1: lo") {
		t.Fatalf("Wrong interface in test container: expected [1: lo], got %s", out)
	}

	deleteAllContainers()

	logDone("run - test loopback only exists when networking disabled")
}

// #7851 hostname outside container shows FQDN, inside only shortname
// For testing purposes it is not required to set host's hostname directly
// and use "--net=host" (as the original issue submitter did), as the same
// codepath is executed with "docker run -h <hostname>".  Both were manually
// tested, but this testcase takes the simpler path of using "run -h .."
func TestRunFullHostnameSet(t *testing.T) {
	cmd := exec.Command(dockerBinary, "run", "-h", "foo.bar.baz", "busybox", "hostname")
	out, _, err := runCommandWithOutput(cmd)
	if err != nil {
		t.Fatal(err, out)
	}

	if actual := strings.Trim(out, "\r\n"); actual != "foo.bar.baz" {
		t.Fatalf("expected hostname 'foo.bar.baz', received %s", actual)
	}
	deleteAllContainers()

	logDone("run - test fully qualified hostname set with -h")
}

func TestRunPrivilegedCanMknod(t *testing.T) {
	cmd := exec.Command(dockerBinary, "run", "--privileged", "busybox", "sh", "-c", "mknod /tmp/sda b 8 0 && echo ok")
	out, _, err := runCommandWithOutput(cmd)
	if err != nil {
		t.Fatal(err)
	}

	if actual := strings.Trim(out, "\r\n"); actual != "ok" {
		t.Fatalf("expected output ok received %s", actual)
	}
	deleteAllContainers()

	logDone("run - test privileged can mknod")
}

func TestRunUnPrivilegedCanMknod(t *testing.T) {
	cmd := exec.Command(dockerBinary, "run", "busybox", "sh", "-c", "mknod /tmp/sda b 8 0 && echo ok")
	out, _, err := runCommandWithOutput(cmd)
	if err != nil {
		t.Fatal(err)
	}

	if actual := strings.Trim(out, "\r\n"); actual != "ok" {
		t.Fatalf("expected output ok received %s", actual)
	}
	deleteAllContainers()

	logDone("run - test un-privileged can mknod")
}

func TestRunCapDropInvalid(t *testing.T) {
	defer deleteAllContainers()
	cmd := exec.Command(dockerBinary, "run", "--cap-drop=CHPASS", "busybox", "ls")
	out, _, err := runCommandWithOutput(cmd)
	if err == nil {
		t.Fatal(err, out)
	}

	logDone("run - test --cap-drop=CHPASS invalid")
}

func TestRunCapDropCannotMknod(t *testing.T) {
	cmd := exec.Command(dockerBinary, "run", "--cap-drop=MKNOD", "busybox", "sh", "-c", "mknod /tmp/sda b 8 0 && echo ok")
	out, _, err := runCommandWithOutput(cmd)
	if err == nil {
		t.Fatal(err, out)
	}

	if actual := strings.Trim(out, "\r\n"); actual == "ok" {
		t.Fatalf("expected output not ok received %s", actual)
	}
	deleteAllContainers()

	logDone("run - test --cap-drop=MKNOD cannot mknod")
}

func TestRunCapDropCannotMknodLowerCase(t *testing.T) {
	cmd := exec.Command(dockerBinary, "run", "--cap-drop=mknod", "busybox", "sh", "-c", "mknod /tmp/sda b 8 0 && echo ok")
	out, _, err := runCommandWithOutput(cmd)
	if err == nil {
		t.Fatal(err, out)
	}

	if actual := strings.Trim(out, "\r\n"); actual == "ok" {
		t.Fatalf("expected output not ok received %s", actual)
	}
	deleteAllContainers()

	logDone("run - test --cap-drop=mknod cannot mknod lowercase")
}

func TestRunCapDropALLCannotMknod(t *testing.T) {
	cmd := exec.Command(dockerBinary, "run", "--cap-drop=ALL", "busybox", "sh", "-c", "mknod /tmp/sda b 8 0 && echo ok")
	out, _, err := runCommandWithOutput(cmd)
	if err == nil {
		t.Fatal(err, out)
	}

	if actual := strings.Trim(out, "\r\n"); actual == "ok" {
		t.Fatalf("expected output not ok received %s", actual)
	}
	deleteAllContainers()

	logDone("run - test --cap-drop=ALL cannot mknod")
}

func TestRunCapDropALLAddMknodCannotMknod(t *testing.T) {
	cmd := exec.Command(dockerBinary, "run", "--cap-drop=ALL", "--cap-add=MKNOD", "busybox", "sh", "-c", "mknod /tmp/sda b 8 0 && echo ok")
	out, _, err := runCommandWithOutput(cmd)
	if err != nil {
		t.Fatal(err, out)
	}

	if actual := strings.Trim(out, "\r\n"); actual != "ok" {
		t.Fatalf("expected output ok received %s", actual)
	}
	deleteAllContainers()

	logDone("run - test --cap-drop=ALL --cap-add=MKNOD can mknod")
}

func TestRunCapAddInvalid(t *testing.T) {
	defer deleteAllContainers()

	cmd := exec.Command(dockerBinary, "run", "--cap-add=CHPASS", "busybox", "ls")
	out, _, err := runCommandWithOutput(cmd)
	if err == nil {
		t.Fatal(err, out)
	}

	logDone("run - test --cap-add=CHPASS invalid")
}

func TestRunCapAddCanDownInterface(t *testing.T) {
	cmd := exec.Command(dockerBinary, "run", "--cap-add=NET_ADMIN", "busybox", "sh", "-c", "ip link set eth0 down && echo ok")
	out, _, err := runCommandWithOutput(cmd)
	if err != nil {
		t.Fatal(err, out)
	}

	if actual := strings.Trim(out, "\r\n"); actual != "ok" {
		t.Fatalf("expected output ok received %s", actual)
	}
	deleteAllContainers()

	logDone("run - test --cap-add=NET_ADMIN can set eth0 down")
}

func TestRunCapAddALLCanDownInterface(t *testing.T) {
	cmd := exec.Command(dockerBinary, "run", "--cap-add=ALL", "busybox", "sh", "-c", "ip link set eth0 down && echo ok")
	out, _, err := runCommandWithOutput(cmd)
	if err != nil {
		t.Fatal(err, out)
	}

	if actual := strings.Trim(out, "\r\n"); actual != "ok" {
		t.Fatalf("expected output ok received %s", actual)
	}
	deleteAllContainers()

	logDone("run - test --cap-add=ALL can set eth0 down")
}

func TestRunCapAddALLDropNetAdminCanDownInterface(t *testing.T) {
	cmd := exec.Command(dockerBinary, "run", "--cap-add=ALL", "--cap-drop=NET_ADMIN", "busybox", "sh", "-c", "ip link set eth0 down && echo ok")
	out, _, err := runCommandWithOutput(cmd)
	if err == nil {
		t.Fatal(err, out)
	}

	if actual := strings.Trim(out, "\r\n"); actual == "ok" {
		t.Fatalf("expected output not ok received %s", actual)
	}
	deleteAllContainers()

	logDone("run - test --cap-add=ALL --cap-drop=NET_ADMIN cannot set eth0 down")
}

func TestRunPrivilegedCanMount(t *testing.T) {
	cmd := exec.Command(dockerBinary, "run", "--privileged", "busybox", "sh", "-c", "mount -t tmpfs none /tmp && echo ok")

	out, _, err := runCommandWithOutput(cmd)
	if err != nil {
		t.Fatal(err)
	}

	if actual := strings.Trim(out, "\r\n"); actual != "ok" {
		t.Fatalf("expected output ok received %s", actual)
	}
	deleteAllContainers()

	logDone("run - test privileged can mount")
}

func TestRunUnPrivilegedCannotMount(t *testing.T) {
	cmd := exec.Command(dockerBinary, "run", "busybox", "sh", "-c", "mount -t tmpfs none /tmp && echo ok")

	out, _, err := runCommandWithOutput(cmd)
	if err == nil {
		t.Fatal(err, out)
	}

	if actual := strings.Trim(out, "\r\n"); actual == "ok" {
		t.Fatalf("expected output not ok received %s", actual)
	}
	deleteAllContainers()

	logDone("run - test un-privileged cannot mount")
}

func TestRunSysNotWritableInNonPrivilegedContainers(t *testing.T) {
	cmd := exec.Command(dockerBinary, "run", "busybox", "touch", "/sys/kernel/profiling")
	if code, err := runCommand(cmd); err == nil || code == 0 {
		t.Fatal("sys should not be writable in a non privileged container")
	}

	deleteAllContainers()

	logDone("run - sys not writable in non privileged container")
}

func TestRunSysWritableInPrivilegedContainers(t *testing.T) {
	cmd := exec.Command(dockerBinary, "run", "--privileged", "busybox", "touch", "/sys/kernel/profiling")
	if code, err := runCommand(cmd); err != nil || code != 0 {
		t.Fatalf("sys should be writable in privileged container")
	}

	deleteAllContainers()

	logDone("run - sys writable in privileged container")
}

func TestRunProcNotWritableInNonPrivilegedContainers(t *testing.T) {
	cmd := exec.Command(dockerBinary, "run", "busybox", "touch", "/proc/sysrq-trigger")
	if code, err := runCommand(cmd); err == nil || code == 0 {
		t.Fatal("proc should not be writable in a non privileged container")
	}

	deleteAllContainers()

	logDone("run - proc not writable in non privileged container")
}

func TestRunProcWritableInPrivilegedContainers(t *testing.T) {
	cmd := exec.Command(dockerBinary, "run", "--privileged", "busybox", "touch", "/proc/sysrq-trigger")
	if code, err := runCommand(cmd); err != nil || code != 0 {
		t.Fatalf("proc should be writable in privileged container")
	}

	deleteAllContainers()

	logDone("run - proc writable in privileged container")
}

func TestRunWithCpuset(t *testing.T) {
	cmd := exec.Command(dockerBinary, "run", "--cpuset", "0", "busybox", "true")
	if code, err := runCommand(cmd); err != nil || code != 0 {
		t.Fatalf("container should run successfuly with cpuset of 0: %s", err)
	}

	deleteAllContainers()

	logDone("run - cpuset 0")
}

func TestRunDeviceNumbers(t *testing.T) {
	cmd := exec.Command(dockerBinary, "run", "busybox", "sh", "-c", "ls -l /dev/null")

	out, _, err := runCommandWithOutput(cmd)
	if err != nil {
		t.Fatal(err, out)
	}
	deviceLineFields := strings.Fields(out)
	deviceLineFields[6] = ""
	deviceLineFields[7] = ""
	deviceLineFields[8] = ""
	expected := []string{"crw-rw-rw-", "1", "root", "root", "1,", "3", "", "", "", "/dev/null"}

	if !(reflect.DeepEqual(deviceLineFields, expected)) {
		t.Fatalf("expected output\ncrw-rw-rw- 1 root root 1, 3 May 24 13:29 /dev/null\n received\n %s\n", out)
	}
	deleteAllContainers()

	logDone("run - test device numbers")
}

func TestRunThatCharacterDevicesActLikeCharacterDevices(t *testing.T) {
	cmd := exec.Command(dockerBinary, "run", "busybox", "sh", "-c", "dd if=/dev/zero of=/zero bs=1k count=5 2> /dev/null ; du -h /zero")

	out, _, err := runCommandWithOutput(cmd)
	if err != nil {
		t.Fatal(err, out)
	}

	if actual := strings.Trim(out, "\r\n"); actual[0] == '0' {
		t.Fatalf("expected a new file called /zero to be create that is greater than 0 bytes long, but du says: %s", actual)
	}
	deleteAllContainers()

	logDone("run - test that character devices work.")
}

func TestRunUnprivilegedWithChroot(t *testing.T) {
	cmd := exec.Command(dockerBinary, "run", "busybox", "chroot", "/", "true")

	if _, err := runCommand(cmd); err != nil {
		t.Fatal(err)
	}

	deleteAllContainers()

	logDone("run - unprivileged with chroot")
}

func TestRunAddingOptionalDevices(t *testing.T) {
	cmd := exec.Command(dockerBinary, "run", "--device", "/dev/zero:/dev/nulo", "busybox", "sh", "-c", "ls /dev/nulo")

	out, _, err := runCommandWithOutput(cmd)
	if err != nil {
		t.Fatal(err, out)
	}

	if actual := strings.Trim(out, "\r\n"); actual != "/dev/nulo" {
		t.Fatalf("expected output /dev/nulo, received %s", actual)
	}
	deleteAllContainers()

	logDone("run - test --device argument")
}

func TestRunModeHostname(t *testing.T) {
	cmd := exec.Command(dockerBinary, "run", "-h=testhostname", "busybox", "cat", "/etc/hostname")

	out, _, err := runCommandWithOutput(cmd)
	if err != nil {
		t.Fatal(err, out)
	}

	if actual := strings.Trim(out, "\r\n"); actual != "testhostname" {
		t.Fatalf("expected 'testhostname', but says: %q", actual)
	}

	cmd = exec.Command(dockerBinary, "run", "--net=host", "busybox", "cat", "/etc/hostname")

	out, _, err = runCommandWithOutput(cmd)
	if err != nil {
		t.Fatal(err, out)
	}
	hostname, err := os.Hostname()
	if err != nil {
		t.Fatal(err)
	}
	if actual := strings.Trim(out, "\r\n"); actual != hostname {
		t.Fatalf("expected %q, but says: %q", hostname, actual)
	}

	deleteAllContainers()

	logDone("run - hostname and several network modes")
}

func TestRunRootWorkdir(t *testing.T) {
	s, _, err := dockerCmd(t, "run", "--workdir", "/", "busybox", "pwd")
	if err != nil {
		t.Fatal(s, err)
	}
	if s != "/\n" {
		t.Fatalf("pwd returned %q (expected /\\n)", s)
	}

	deleteAllContainers()

	logDone("run - workdir /")
}

func TestRunAllowBindMountingRoot(t *testing.T) {
	s, _, err := dockerCmd(t, "run", "-v", "/:/host", "busybox", "ls", "/host")
	if err != nil {
		t.Fatal(s, err)
	}

	deleteAllContainers()

	logDone("run - bind mount / as volume")
}

func TestRunDisallowBindMountingRootToRoot(t *testing.T) {
	cmd := exec.Command(dockerBinary, "run", "-v", "/:/", "busybox", "ls", "/host")
	out, _, err := runCommandWithOutput(cmd)
	if err == nil {
		t.Fatal(out, err)
	}

	deleteAllContainers()

	logDone("run - bind mount /:/ as volume should fail")
}

// Test recursive bind mount works by default
func TestRunWithVolumesIsRecursive(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "docker_recursive_mount_test")
	if err != nil {
		t.Fatal(err)
	}

	defer os.RemoveAll(tmpDir)

	// Create a temporary tmpfs mount.
	tmpfsDir := filepath.Join(tmpDir, "tmpfs")
	if err := os.MkdirAll(tmpfsDir, 0777); err != nil {
		t.Fatalf("failed to mkdir at %s - %s", tmpfsDir, err)
	}
	if err := mount.Mount("tmpfs", tmpfsDir, "tmpfs", ""); err != nil {
		t.Fatalf("failed to create a tmpfs mount at %s - %s", tmpfsDir, err)
	}
	defer mount.Unmount(tmpfsDir)

	f, err := ioutil.TempFile(tmpfsDir, "touch-me")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	runCmd := exec.Command(dockerBinary, "run", "--name", "test-data", "--volume", fmt.Sprintf("%s:/tmp:ro", tmpDir), "busybox:latest", "ls", "/tmp/tmpfs")
	out, stderr, exitCode, err := runCommandWithStdoutStderr(runCmd)
	if err != nil && exitCode != 0 {
		t.Fatal(out, stderr, err)
	}
	if !strings.Contains(out, filepath.Base(f.Name())) {
		t.Fatal("Recursive bind mount test failed. Expected file not found")
	}

	deleteAllContainers()

	logDone("run - volumes are bind mounted recursively")
}

func TestRunDnsDefaultOptions(t *testing.T) {
	// ci server has default resolv.conf
	// so rewrite it for the test
	origResolvConf, err := ioutil.ReadFile("/etc/resolv.conf")
	if os.IsNotExist(err) {
		t.Fatalf("/etc/resolv.conf does not exist")
	}

	// test with file
	tmpResolvConf := []byte("nameserver 127.0.0.1")
	if err := ioutil.WriteFile("/etc/resolv.conf", tmpResolvConf, 0644); err != nil {
		t.Fatal(err)
	}
	// put the old resolvconf back
	defer func() {
		if err := ioutil.WriteFile("/etc/resolv.conf", origResolvConf, 0644); err != nil {
			t.Fatal(err)
		}
	}()

	cmd := exec.Command(dockerBinary, "run", "busybox", "cat", "/etc/resolv.conf")

	actual, _, err := runCommandWithOutput(cmd)
	if err != nil {
		t.Error(err, actual)
		return
	}

	// check that the actual defaults are there
	// if we ever change the defaults from google dns, this will break
	expected := "\nnameserver 8.8.8.8\nnameserver 8.8.4.4"
	if actual != expected {
		t.Errorf("expected resolv.conf be: %q, but was: %q", expected, actual)
		return
	}

	deleteAllContainers()

	logDone("run - dns default options")
}

func TestRunDnsOptions(t *testing.T) {
	cmd := exec.Command(dockerBinary, "run", "--dns=127.0.0.1", "--dns-search=mydomain", "busybox", "cat", "/etc/resolv.conf")

	out, _, err := runCommandWithOutput(cmd)
	if err != nil {
		t.Fatal(err, out)
	}

	actual := strings.Replace(strings.Trim(out, "\r\n"), "\n", " ", -1)
	if actual != "nameserver 127.0.0.1 search mydomain" {
		t.Fatalf("expected 'nameserver 127.0.0.1 search mydomain', but says: %q", actual)
	}

	cmd = exec.Command(dockerBinary, "run", "--dns=127.0.0.1", "--dns-search=.", "busybox", "cat", "/etc/resolv.conf")

	out, _, err = runCommandWithOutput(cmd)
	if err != nil {
		t.Fatal(err, out)
	}

	actual = strings.Replace(strings.Trim(strings.Trim(out, "\r\n"), " "), "\n", " ", -1)
	if actual != "nameserver 127.0.0.1" {
		t.Fatalf("expected 'nameserver 127.0.0.1', but says: %q", actual)
	}

	logDone("run - dns options")
}

func TestRunDnsOptionsBasedOnHostResolvConf(t *testing.T) {
	var out string

	origResolvConf, err := ioutil.ReadFile("/etc/resolv.conf")
	if os.IsNotExist(err) {
		t.Fatalf("/etc/resolv.conf does not exist")
	}

	hostNamservers := resolvconf.GetNameservers(origResolvConf)
	hostSearch := resolvconf.GetSearchDomains(origResolvConf)

	cmd := exec.Command(dockerBinary, "run", "--dns=127.0.0.1", "busybox", "cat", "/etc/resolv.conf")

	if out, _, err = runCommandWithOutput(cmd); err != nil {
		t.Fatal(err, out)
	}

	if actualNameservers := resolvconf.GetNameservers([]byte(out)); string(actualNameservers[0]) != "127.0.0.1" {
		t.Fatalf("expected '127.0.0.1', but says: %q", string(actualNameservers[0]))
	}

	actualSearch := resolvconf.GetSearchDomains([]byte(out))
	if len(actualSearch) != len(hostSearch) {
		t.Fatalf("expected %q search domain(s), but it has: %q", len(hostSearch), len(actualSearch))
	}
	for i := range actualSearch {
		if actualSearch[i] != hostSearch[i] {
			t.Fatalf("expected %q domain, but says: %q", actualSearch[i], hostSearch[i])
		}
	}

	cmd = exec.Command(dockerBinary, "run", "--dns-search=mydomain", "busybox", "cat", "/etc/resolv.conf")

	if out, _, err = runCommandWithOutput(cmd); err != nil {
		t.Fatal(err, out)
	}

	actualNameservers := resolvconf.GetNameservers([]byte(out))
	if len(actualNameservers) != len(hostNamservers) {
		t.Fatalf("expected %q nameserver(s), but it has: %q", len(hostNamservers), len(actualNameservers))
	}
	for i := range actualNameservers {
		if actualNameservers[i] != hostNamservers[i] {
			t.Fatalf("expected %q nameserver, but says: %q", actualNameservers[i], hostNamservers[i])
		}
	}

	if actualSearch = resolvconf.GetSearchDomains([]byte(out)); string(actualSearch[0]) != "mydomain" {
		t.Fatalf("expected 'mydomain', but says: %q", string(actualSearch[0]))
	}

	// test with file
	tmpResolvConf := []byte("search example.com\nnameserver 12.34.56.78\nnameserver 127.0.0.1")
	if err := ioutil.WriteFile("/etc/resolv.conf", tmpResolvConf, 0644); err != nil {
		t.Fatal(err)
	}
	// put the old resolvconf back
	defer func() {
		if err := ioutil.WriteFile("/etc/resolv.conf", origResolvConf, 0644); err != nil {
			t.Fatal(err)
		}
	}()

	resolvConf, err := ioutil.ReadFile("/etc/resolv.conf")
	if os.IsNotExist(err) {
		t.Fatalf("/etc/resolv.conf does not exist")
	}

	hostNamservers = resolvconf.GetNameservers(resolvConf)
	hostSearch = resolvconf.GetSearchDomains(resolvConf)

	cmd = exec.Command(dockerBinary, "run", "busybox", "cat", "/etc/resolv.conf")

	if out, _, err = runCommandWithOutput(cmd); err != nil {
		t.Fatal(err, out)
	}

	if actualNameservers = resolvconf.GetNameservers([]byte(out)); string(actualNameservers[0]) != "12.34.56.78" || len(actualNameservers) != 1 {
		t.Fatalf("expected '12.34.56.78', but has: %v", actualNameservers)
	}

	actualSearch = resolvconf.GetSearchDomains([]byte(out))
	if len(actualSearch) != len(hostSearch) {
		t.Fatalf("expected %q search domain(s), but it has: %q", len(hostSearch), len(actualSearch))
	}
	for i := range actualSearch {
		if actualSearch[i] != hostSearch[i] {
			t.Fatalf("expected %q domain, but says: %q", actualSearch[i], hostSearch[i])
		}
	}

	deleteAllContainers()

	logDone("run - dns options based on host resolv.conf")
}

func TestRunAddHost(t *testing.T) {
	defer deleteAllContainers()
	cmd := exec.Command(dockerBinary, "run", "--add-host=extra:86.75.30.9", "busybox", "grep", "extra", "/etc/hosts")

	out, _, err := runCommandWithOutput(cmd)
	if err != nil {
		t.Fatal(err, out)
	}

	actual := strings.Trim(out, "\r\n")
	if actual != "86.75.30.9\textra" {
		t.Fatalf("expected '86.75.30.9\textra', but says: %q", actual)
	}

	logDone("run - add-host option")
}

// Regression test for #6983
func TestRunAttachStdErrOnlyTTYMode(t *testing.T) {
	cmd := exec.Command(dockerBinary, "run", "-t", "-a", "stderr", "busybox", "true")

	exitCode, err := runCommand(cmd)
	if err != nil {
		t.Fatal(err)
	} else if exitCode != 0 {
		t.Fatalf("Container should have exited with error code 0")
	}

	deleteAllContainers()

	logDone("run - Attach stderr only with -t")
}

// Regression test for #6983
func TestRunAttachStdOutOnlyTTYMode(t *testing.T) {
	cmd := exec.Command(dockerBinary, "run", "-t", "-a", "stdout", "busybox", "true")

	exitCode, err := runCommand(cmd)
	if err != nil {
		t.Fatal(err)
	} else if exitCode != 0 {
		t.Fatalf("Container should have exited with error code 0")
	}

	deleteAllContainers()

	logDone("run - Attach stdout only with -t")
}

// Regression test for #6983
func TestRunAttachStdOutAndErrTTYMode(t *testing.T) {
	cmd := exec.Command(dockerBinary, "run", "-t", "-a", "stdout", "-a", "stderr", "busybox", "true")

	exitCode, err := runCommand(cmd)
	if err != nil {
		t.Fatal(err)
	} else if exitCode != 0 {
		t.Fatalf("Container should have exited with error code 0")
	}

	deleteAllContainers()

	logDone("run - Attach stderr and stdout with -t")
}

func TestRunState(t *testing.T) {
	defer deleteAllContainers()
	cmd := exec.Command(dockerBinary, "run", "-d", "busybox", "top")

	out, _, err := runCommandWithOutput(cmd)
	if err != nil {
		t.Fatal(err, out)
	}
	id := strings.TrimSpace(out)
	state, err := inspectField(id, "State.Running")
	if err != nil {
		t.Fatal(err)
	}
	if state != "true" {
		t.Fatal("Container state is 'not running'")
	}
	pid1, err := inspectField(id, "State.Pid")
	if err != nil {
		t.Fatal(err)
	}
	if pid1 == "0" {
		t.Fatal("Container state Pid 0")
	}

	cmd = exec.Command(dockerBinary, "stop", id)
	out, _, err = runCommandWithOutput(cmd)
	if err != nil {
		t.Fatal(err, out)
	}
	state, err = inspectField(id, "State.Running")
	if err != nil {
		t.Fatal(err)
	}
	if state != "false" {
		t.Fatal("Container state is 'running'")
	}
	pid2, err := inspectField(id, "State.Pid")
	if err != nil {
		t.Fatal(err)
	}
	if pid2 == pid1 {
		t.Fatalf("Container state Pid %s, but expected %s", pid2, pid1)
	}

	cmd = exec.Command(dockerBinary, "start", id)
	out, _, err = runCommandWithOutput(cmd)
	if err != nil {
		t.Fatal(err, out)
	}
	state, err = inspectField(id, "State.Running")
	if err != nil {
		t.Fatal(err)
	}
	if state != "true" {
		t.Fatal("Container state is 'not running'")
	}
	pid3, err := inspectField(id, "State.Pid")
	if err != nil {
		t.Fatal(err)
	}
	if pid3 == pid1 {
		t.Fatalf("Container state Pid %s, but expected %s", pid2, pid1)
	}
	logDone("run - test container state.")
}

// Test for #1737
func TestRunCopyVolumeUidGid(t *testing.T) {
	name := "testrunvolumesuidgid"
	defer deleteImages(name)
	defer deleteAllContainers()
	_, err := buildImage(name,
		`FROM busybox
		RUN echo 'dockerio:x:1001:1001::/bin:/bin/false' >> /etc/passwd
		RUN echo 'dockerio:x:1001:' >> /etc/group
		RUN mkdir -p /hello && touch /hello/test && chown dockerio.dockerio /hello`,
		true)
	if err != nil {
		t.Fatal(err)
	}

	// Test that the uid and gid is copied from the image to the volume
	cmd := exec.Command(dockerBinary, "run", "--rm", "-v", "/hello", name, "sh", "-c", "ls -l / | grep hello | awk '{print $3\":\"$4}'")
	out, _, err := runCommandWithOutput(cmd)
	if err != nil {
		t.Fatal(err, out)
	}
	out = strings.TrimSpace(out)
	if out != "dockerio:dockerio" {
		t.Fatalf("Wrong /hello ownership: %s, expected dockerio:dockerio", out)
	}

	logDone("run - copy uid/gid for volume")
}

// Test for #1582
func TestRunCopyVolumeContent(t *testing.T) {
	name := "testruncopyvolumecontent"
	defer deleteImages(name)
	defer deleteAllContainers()
	_, err := buildImage(name,
		`FROM busybox
		RUN mkdir -p /hello/local && echo hello > /hello/local/world`,
		true)
	if err != nil {
		t.Fatal(err)
	}

	// Test that the content is copied from the image to the volume
	cmd := exec.Command(dockerBinary, "run", "--rm", "-v", "/hello", name, "find", "/hello")
	out, _, err := runCommandWithOutput(cmd)
	if err != nil {
		t.Fatal(err, out)
	}
	if !(strings.Contains(out, "/hello/local/world") && strings.Contains(out, "/hello/local")) {
		t.Fatal("Container failed to transfer content to volume")
	}
	logDone("run - copy volume content")
}

func TestRunCleanupCmdOnEntrypoint(t *testing.T) {
	name := "testrunmdcleanuponentrypoint"
	defer deleteImages(name)
	defer deleteAllContainers()
	if _, err := buildImage(name,
		`FROM busybox
		ENTRYPOINT ["echo"]
        CMD ["testingpoint"]`,
		true); err != nil {
		t.Fatal(err)
	}
	runCmd := exec.Command(dockerBinary, "run", "--entrypoint", "whoami", name)
	out, exit, err := runCommandWithOutput(runCmd)
	if err != nil {
		t.Fatalf("Error: %v, out: %q", err, out)
	}
	if exit != 0 {
		t.Fatalf("expected exit code 0 received %d, out: %q", exit, out)
	}
	out = strings.TrimSpace(out)
	if out != "root" {
		t.Fatalf("Expected output root, got %q", out)
	}
	logDone("run - cleanup cmd on --entrypoint")
}

// TestRunWorkdirExistsAndIsFile checks that if 'docker run -w' with existing file can be detected
func TestRunWorkdirExistsAndIsFile(t *testing.T) {
	defer deleteAllContainers()
	runCmd := exec.Command(dockerBinary, "run", "-w", "/bin/cat", "busybox")
	out, exit, err := runCommandWithOutput(runCmd)
	if !(err != nil && exit == 1 && strings.Contains(out, "Cannot mkdir: /bin/cat is not a directory")) {
		t.Fatalf("Docker must complains about making dir, but we got out: %s, exit: %d, err: %s", out, exit, err)
	}
	logDone("run - error on existing file for workdir")
}

func TestRunExitOnStdinClose(t *testing.T) {
	name := "testrunexitonstdinclose"
	defer deleteAllContainers()
	runCmd := exec.Command(dockerBinary, "run", "--name", name, "-i", "busybox", "/bin/cat")

	stdin, err := runCmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := runCmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}

	if err := runCmd.Start(); err != nil {
		t.Fatal(err)
	}
	if _, err := stdin.Write([]byte("hello\n")); err != nil {
		t.Fatal(err)
	}

	r := bufio.NewReader(stdout)
	line, err := r.ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	line = strings.TrimSpace(line)
	if line != "hello" {
		t.Fatalf("Output should be 'hello', got '%q'", line)
	}
	if err := stdin.Close(); err != nil {
		t.Fatal(err)
	}
	finish := make(chan struct{})
	go func() {
		if err := runCmd.Wait(); err != nil {
			t.Fatal(err)
		}
		close(finish)
	}()
	select {
	case <-finish:
	case <-time.After(1 * time.Second):
		t.Fatal("docker run failed to exit on stdin close")
	}
	state, err := inspectField(name, "State.Running")
	if err != nil {
		t.Fatal(err)
	}
	if state != "false" {
		t.Fatal("Container must be stopped after stdin closing")
	}
	logDone("run - exit on stdin closing")
}

// Test for #2267
func TestRunWriteHostsFileAndNotCommit(t *testing.T) {
	defer deleteAllContainers()

	name := "writehosts"
	cmd := exec.Command(dockerBinary, "run", "--name", name, "busybox", "sh", "-c", "echo test2267 >> /etc/hosts && cat /etc/hosts")
	out, _, err := runCommandWithOutput(cmd)
	if err != nil {
		t.Fatal(err, out)
	}
	if !strings.Contains(out, "test2267") {
		t.Fatal("/etc/hosts should contain 'test2267'")
	}

	cmd = exec.Command(dockerBinary, "diff", name)
	if err != nil {
		t.Fatal(err, out)
	}
	out, _, err = runCommandWithOutput(cmd)
	if err != nil {
		t.Fatal(err, out)
	}
	if len(strings.Trim(out, "\r\n")) != 0 {
		t.Fatal("diff should be empty")
	}

	logDone("run - write to /etc/hosts and not commited")
}

// Test for #2267
func TestRunWriteHostnameFileAndNotCommit(t *testing.T) {
	defer deleteAllContainers()

	name := "writehostname"
	cmd := exec.Command(dockerBinary, "run", "--name", name, "busybox", "sh", "-c", "echo test2267 >> /etc/hostname && cat /etc/hostname")
	out, _, err := runCommandWithOutput(cmd)
	if err != nil {
		t.Fatal(err, out)
	}
	if !strings.Contains(out, "test2267") {
		t.Fatal("/etc/hostname should contain 'test2267'")
	}

	cmd = exec.Command(dockerBinary, "diff", name)
	if err != nil {
		t.Fatal(err, out)
	}
	out, _, err = runCommandWithOutput(cmd)
	if err != nil {
		t.Fatal(err, out)
	}
	if len(strings.Trim(out, "\r\n")) != 0 {
		t.Fatal("diff should be empty")
	}

	logDone("run - write to /etc/hostname and not commited")
}

// Test for #2267
func TestRunWriteResolvFileAndNotCommit(t *testing.T) {
	defer deleteAllContainers()

	name := "writeresolv"
	cmd := exec.Command(dockerBinary, "run", "--name", name, "busybox", "sh", "-c", "echo test2267 >> /etc/resolv.conf && cat /etc/resolv.conf")
	out, _, err := runCommandWithOutput(cmd)
	if err != nil {
		t.Fatal(err, out)
	}
	if !strings.Contains(out, "test2267") {
		t.Fatal("/etc/resolv.conf should contain 'test2267'")
	}

	cmd = exec.Command(dockerBinary, "diff", name)
	if err != nil {
		t.Fatal(err, out)
	}
	out, _, err = runCommandWithOutput(cmd)
	if err != nil {
		t.Fatal(err, out)
	}
	if len(strings.Trim(out, "\r\n")) != 0 {
		t.Fatal("diff should be empty")
	}

	logDone("run - write to /etc/resolv.conf and not commited")
}

func TestRunWithBadDevice(t *testing.T) {
	defer deleteAllContainers()

	name := "baddevice"
	cmd := exec.Command(dockerBinary, "run", "--name", name, "--device", "/etc", "busybox", "true")
	out, _, err := runCommandWithOutput(cmd)
	if err == nil {
		t.Fatal("Run should fail with bad device")
	}
	expected := `\"/etc\": not a device node`
	if !strings.Contains(out, expected) {
		t.Fatalf("Output should contain %q, actual out: %q", expected, out)
	}
	logDone("run - error with bad device")
}

func TestRunEntrypoint(t *testing.T) {
	defer deleteAllContainers()

	name := "entrypoint"
	cmd := exec.Command(dockerBinary, "run", "--name", name, "--entrypoint", "/bin/echo", "busybox", "-n", "foobar")
	out, _, err := runCommandWithOutput(cmd)
	if err != nil {
		t.Fatal(err, out)
	}
	expected := "foobar"
	if out != expected {
		t.Fatalf("Output should be %q, actual out: %q", expected, out)
	}
	logDone("run - entrypoint")
}

func TestRunBindMounts(t *testing.T) {
	defer deleteAllContainers()

	tmpDir, err := ioutil.TempDir("", "docker-test-container")
	if err != nil {
		t.Fatal(err)
	}

	defer os.RemoveAll(tmpDir)
	writeFile(path.Join(tmpDir, "touch-me"), "", t)

	// Test reading from a read-only bind mount
	cmd := exec.Command(dockerBinary, "run", "-v", fmt.Sprintf("%s:/tmp:ro", tmpDir), "busybox", "ls", "/tmp")
	out, _, err := runCommandWithOutput(cmd)
	if err != nil {
		t.Fatal(err, out)
	}
	if !strings.Contains(out, "touch-me") {
		t.Fatal("Container failed to read from bind mount")
	}

	// test writing to bind mount
	cmd = exec.Command(dockerBinary, "run", "-v", fmt.Sprintf("%s:/tmp:rw", tmpDir), "busybox", "touch", "/tmp/holla")
	out, _, err = runCommandWithOutput(cmd)
	if err != nil {
		t.Fatal(err, out)
	}
	readFile(path.Join(tmpDir, "holla"), t) // Will fail if the file doesn't exist

	// test mounting to an illegal destination directory
	cmd = exec.Command(dockerBinary, "run", "-v", fmt.Sprintf("%s:.", tmpDir), "busybox", "ls", ".")
	_, err = runCommand(cmd)
	if err == nil {
		t.Fatal("Container bind mounted illegal directory")
	}

	// test mount a file
	cmd = exec.Command(dockerBinary, "run", "-v", fmt.Sprintf("%s/holla:/tmp/holla:rw", tmpDir), "busybox", "sh", "-c", "echo -n 'yotta' > /tmp/holla")
	_, err = runCommand(cmd)
	if err != nil {
		t.Fatal(err, out)
	}
	content := readFile(path.Join(tmpDir, "holla"), t) // Will fail if the file doesn't exist
	expected := "yotta"
	if content != expected {
		t.Fatalf("Output should be %q, actual out: %q", expected, content)
	}

	logDone("run - bind mounts")
}

func TestRunMutableNetworkFiles(t *testing.T) {
	defer deleteAllContainers()

	for _, fn := range []string{"resolv.conf", "hosts"} {
		deleteAllContainers()

		content, err := runCommandAndReadContainerFile(fn, exec.Command(dockerBinary, "run", "-d", "--name", "c1", "busybox", "sh", "-c", fmt.Sprintf("echo success >/etc/%s; while true; do sleep 1; done", fn)))
		if err != nil {
			t.Fatal(err)
		}

		if strings.TrimSpace(string(content)) != "success" {
			t.Fatal("Content was not what was modified in the container", string(content))
		}

		out, _, err := runCommandWithOutput(exec.Command(dockerBinary, "run", "-d", "--name", "c2", "busybox", "sh", "-c", fmt.Sprintf("while true; do cat /etc/%s; sleep 1; done", fn)))
		if err != nil {
			t.Fatal(err)
		}

		contID := strings.TrimSpace(out)

		resolvConfPath := containerStorageFile(contID, fn)

		f, err := os.OpenFile(resolvConfPath, os.O_WRONLY|os.O_SYNC|os.O_APPEND, 0644)
		if err != nil {
			t.Fatal(err)
		}

		if _, err := f.Seek(0, 0); err != nil {
			f.Close()
			t.Fatal(err)
		}

		if err := f.Truncate(0); err != nil {
			f.Close()
			t.Fatal(err)
		}

		if _, err := f.Write([]byte("success2\n")); err != nil {
			f.Close()
			t.Fatal(err)
		}

		f.Close()

		time.Sleep(2 * time.Second) // don't race sleep

		out, _, err = runCommandWithOutput(exec.Command(dockerBinary, "logs", "c2"))
		if err != nil {
			t.Fatal(err)
		}

		lines := strings.Split(out, "\n")
		if strings.TrimSpace(lines[len(lines)-2]) != "success2" {
			t.Fatalf("Did not find the correct output in /etc/%s: %s %#v", fn, out, lines)
		}
	}
}

// Ensure that CIDFile gets deleted if it's empty
// Perform this test by making `docker run` fail
func TestRunCidFileCleanupIfEmpty(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "TestRunCidFile")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)
	tmpCidFile := path.Join(tmpDir, "cid")
	cmd := exec.Command(dockerBinary, "run", "--cidfile", tmpCidFile, "scratch")
	out, _, err := runCommandWithOutput(cmd)
	t.Log(out)
	if err == nil {
		t.Fatal("Run without command must fail")
	}

	if _, err := os.Stat(tmpCidFile); err == nil {
		t.Fatalf("empty CIDFile %q should've been deleted", tmpCidFile)
	}
	deleteAllContainers()
	logDone("run - cleanup empty cidfile on fail")
}

// #2098 - Docker cidFiles only contain short version of the containerId
//sudo docker run --cidfile /tmp/docker_test.cid ubuntu echo "test"
// TestRunCidFile tests that run --cidfile returns the longid
func TestRunCidFileCheckIDLength(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "TestRunCidFile")
	if err != nil {
		t.Fatal(err)
	}
	tmpCidFile := path.Join(tmpDir, "cid")
	defer os.RemoveAll(tmpDir)
	cmd := exec.Command(dockerBinary, "run", "-d", "--cidfile", tmpCidFile, "busybox", "true")
	out, _, err := runCommandWithOutput(cmd)
	if err != nil {
		t.Fatal(err)
	}
	id := strings.TrimSpace(out)
	buffer, err := ioutil.ReadFile(tmpCidFile)
	if err != nil {
		t.Fatal(err)
	}
	cid := string(buffer)
	if len(cid) != 64 {
		t.Fatalf("--cidfile should be a long id, not %q", id)
	}
	if cid != id {
		t.Fatalf("cid must be equal to %s, got %s", id, cid)
	}
	deleteAllContainers()
	logDone("run - cidfile contains long id")
}

func TestRunNetworkNotInitializedNoneMode(t *testing.T) {
	cmd := exec.Command(dockerBinary, "run", "-d", "--net=none", "busybox", "top")
	out, _, err := runCommandWithOutput(cmd)
	if err != nil {
		t.Fatal(err)
	}
	id := strings.TrimSpace(out)
	res, err := inspectField(id, "NetworkSettings.IPAddress")
	if err != nil {
		t.Fatal(err)
	}
	if res != "" {
		t.Fatalf("For 'none' mode network must not be initialized, but container got IP: %s", res)
	}
	deleteAllContainers()
	logDone("run - network must not be initialized in 'none' mode")
}

func TestRunSetMacAddress(t *testing.T) {
	mac := "12:34:56:78:9a:bc"
	cmd := exec.Command("/bin/bash", "-c", dockerBinary+` run -i --rm --mac-address=`+mac+` busybox /bin/sh -c "ip link show eth0 | tail -1 | awk '{ print \$2 }'"`)
	out, _, err := runCommandWithOutput(cmd)
	if err != nil {
		t.Fatal(err)
	}
	actualMac := strings.TrimSpace(out)
	if actualMac != mac {
		t.Fatalf("Set MAC address with --mac-address failed. The container has an incorrect MAC address: %q, expected: %q", actualMac, mac)
	}

	deleteAllContainers()
	logDone("run - setting MAC address with --mac-address")
}

func TestRunInspectMacAddress(t *testing.T) {
	mac := "12:34:56:78:9a:bc"
	cmd := exec.Command(dockerBinary, "run", "-d", "--mac-address="+mac, "busybox", "top")
	out, _, err := runCommandWithOutput(cmd)
	if err != nil {
		t.Fatal(err)
	}
	id := strings.TrimSpace(out)
	inspectedMac, err := inspectField(id, "NetworkSettings.MacAddress")
	if err != nil {
		t.Fatal(err)
	}
	if inspectedMac != mac {
		t.Fatalf("docker inspect outputs wrong MAC address: %q, should be: %q", inspectedMac, mac)
	}
	deleteAllContainers()
	logDone("run - inspecting MAC address")
}

func TestRunDeallocatePortOnMissingIptablesRule(t *testing.T) {
	cmd := exec.Command(dockerBinary, "run", "-d", "-p", "23:23", "busybox", "top")
	out, _, err := runCommandWithOutput(cmd)
	if err != nil {
		t.Fatal(err)
	}
	id := strings.TrimSpace(out)
	ip, err := inspectField(id, "NetworkSettings.IPAddress")
	if err != nil {
		t.Fatal(err)
	}
	iptCmd := exec.Command("iptables", "-D", "FORWARD", "-d", fmt.Sprintf("%s/32", ip),
		"!", "-i", "docker0", "-o", "docker0", "-p", "tcp", "-m", "tcp", "--dport", "23", "-j", "ACCEPT")
	out, _, err = runCommandWithOutput(iptCmd)
	if err != nil {
		t.Fatal(err, out)
	}
	if err := deleteContainer(id); err != nil {
		t.Fatal(err)
	}
	cmd = exec.Command(dockerBinary, "run", "-d", "-p", "23:23", "busybox", "top")
	out, _, err = runCommandWithOutput(cmd)
	if err != nil {
		t.Fatal(err, out)
	}
	deleteAllContainers()
	logDone("run - port should be deallocated even on iptables error")
}

func TestRunPortInUse(t *testing.T) {
	port := "1234"
	l, err := net.Listen("tcp", ":"+port)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	cmd := exec.Command(dockerBinary, "run", "-d", "-p", port+":80", "busybox", "top")
	out, _, err := runCommandWithOutput(cmd)
	if err == nil {
		t.Fatalf("Binding on used port must fail")
	}
	if !strings.Contains(out, "address already in use") {
		t.Fatalf("Out must be about \"address already in use\", got %s", out)
	}

	deleteAllContainers()
	logDone("run - fail if port already in use")
}

// https://github.com/docker/docker/issues/8428
func TestRunPortProxy(t *testing.T) {
	defer deleteAllContainers()

	port := "12345"
	cmd := exec.Command(dockerBinary, "run", "-d", "-p", port+":80", "busybox", "top")

	out, _, err := runCommandWithOutput(cmd)
	if err != nil {
		t.Fatalf("Failed to run and bind port %s, output: %s, error: %s", port, out, err)
	}

	// connect for 10 times here. This will trigger 10 EPIPES in the child
	// process and kill it when it writes to a closed stdout/stderr
	for i := 0; i < 10; i++ {
		net.Dial("tcp", fmt.Sprintf("0.0.0.0:%s", port))
	}

	listPs := exec.Command("sh", "-c", "ps ax | grep docker")
	out, _, err = runCommandWithOutput(listPs)
	if err != nil {
		t.Errorf("list docker process failed with output %s, error %s", out, err)
	}
	if strings.Contains(out, "docker <defunct>") {
		t.Errorf("Unexpected defunct docker process")
	}
	if !strings.Contains(out, "docker-proxy -proto tcp -host-ip 0.0.0.0 -host-port 12345") {
		t.Errorf("Failed to find docker-proxy process, got %s", out)
	}

	logDone("run - proxy should work with unavailable port")
}

// Regression test for #7792
func TestRunMountOrdering(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "docker_nested_mount_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	tmpDir2, err := ioutil.TempDir("", "docker_nested_mount_test2")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir2)

	// Create a temporary tmpfs mount.
	fooDir := filepath.Join(tmpDir, "foo")
	if err := os.MkdirAll(filepath.Join(tmpDir, "foo"), 0755); err != nil {
		t.Fatalf("failed to mkdir at %s - %s", fooDir, err)
	}

	if err := ioutil.WriteFile(fmt.Sprintf("%s/touch-me", fooDir), []byte{}, 0644); err != nil {
		t.Fatal(err)
	}

	if err := ioutil.WriteFile(fmt.Sprintf("%s/touch-me", tmpDir), []byte{}, 0644); err != nil {
		t.Fatal(err)
	}

	if err := ioutil.WriteFile(fmt.Sprintf("%s/touch-me", tmpDir2), []byte{}, 0644); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(dockerBinary, "run", "-v", fmt.Sprintf("%s:/tmp", tmpDir), "-v", fmt.Sprintf("%s:/tmp/foo", fooDir), "-v", fmt.Sprintf("%s:/tmp/tmp2", tmpDir2), "-v", fmt.Sprintf("%s:/tmp/tmp2/foo", fooDir), "busybox:latest", "sh", "-c", "ls /tmp/touch-me && ls /tmp/foo/touch-me && ls /tmp/tmp2/touch-me && ls /tmp/tmp2/foo/touch-me")
	out, _, err := runCommandWithOutput(cmd)
	if err != nil {
		t.Fatal(out, err)
	}

	deleteAllContainers()
	logDone("run - volumes are mounted in the correct order")
}

func TestRunExecDir(t *testing.T) {
	cmd := exec.Command(dockerBinary, "run", "-d", "busybox", "top")
	out, _, err := runCommandWithOutput(cmd)
	if err != nil {
		t.Fatal(err, out)
	}
	id := strings.TrimSpace(out)
	execDir := filepath.Join(execDriverPath, id)
	stateFile := filepath.Join(execDir, "state.json")
	contFile := filepath.Join(execDir, "container.json")

	{
		fi, err := os.Stat(execDir)
		if err != nil {
			t.Fatal(err)
		}
		if !fi.IsDir() {
			t.Fatalf("%q must be a directory", execDir)
		}
		fi, err = os.Stat(stateFile)
		if err != nil {
			t.Fatal(err)
		}
		fi, err = os.Stat(contFile)
		if err != nil {
			t.Fatal(err)
		}
	}

	stopCmd := exec.Command(dockerBinary, "stop", id)
	out, _, err = runCommandWithOutput(stopCmd)
	if err != nil {
		t.Fatal(err, out)
	}
	{
		fi, err := os.Stat(execDir)
		if err != nil {
			t.Fatal(err)
		}
		if !fi.IsDir() {
			t.Fatalf("%q must be a directory", execDir)
		}
		fi, err = os.Stat(stateFile)
		if err == nil {
			t.Fatalf("Statefile %q is exists for stopped container!", stateFile)
		}
		if !os.IsNotExist(err) {
			t.Fatalf("Error should be about non-existing, got %s", err)
		}
		fi, err = os.Stat(contFile)
		if err == nil {
			t.Fatalf("Container file %q is exists for stopped container!", contFile)
		}
		if !os.IsNotExist(err) {
			t.Fatalf("Error should be about non-existing, got %s", err)
		}
	}
	startCmd := exec.Command(dockerBinary, "start", id)
	out, _, err = runCommandWithOutput(startCmd)
	if err != nil {
		t.Fatal(err, out)
	}
	{
		fi, err := os.Stat(execDir)
		if err != nil {
			t.Fatal(err)
		}
		if !fi.IsDir() {
			t.Fatalf("%q must be a directory", execDir)
		}
		fi, err = os.Stat(stateFile)
		if err != nil {
			t.Fatal(err)
		}
		fi, err = os.Stat(contFile)
		if err != nil {
			t.Fatal(err)
		}
	}
	rmCmd := exec.Command(dockerBinary, "rm", "-f", id)
	out, _, err = runCommandWithOutput(rmCmd)
	if err != nil {
		t.Fatal(err, out)
	}
	{
		_, err := os.Stat(execDir)
		if err == nil {
			t.Fatal(err)
		}
		if err == nil {
			t.Fatalf("Exec directory %q is exists for removed container!", execDir)
		}
		if !os.IsNotExist(err) {
			t.Fatalf("Error should be about non-existing, got %s", err)
		}
	}

	logDone("run - check execdriver dir behavior")
}

// #6509
func TestRunRedirectStdout(t *testing.T) {

	defer deleteAllContainers()

	checkRedirect := func(command string) {
		_, tty, err := pty.Open()
		if err != nil {
			t.Fatalf("Could not open pty: %v", err)
		}
		cmd := exec.Command("sh", "-c", command)
		cmd.Stdin = tty
		cmd.Stdout = tty
		cmd.Stderr = tty
		ch := make(chan struct{})
		if err := cmd.Start(); err != nil {
			t.Fatalf("start err: %v", err)
		}
		go func() {
			if err := cmd.Wait(); err != nil {
				t.Fatalf("wait err=%v", err)
			}
			close(ch)
		}()

		select {
		case <-time.After(10 * time.Second):
			t.Fatal("command timeout")
		case <-ch:
		}
	}

	checkRedirect(dockerBinary + " run -i busybox cat /etc/passwd | grep -q root")
	checkRedirect(dockerBinary + " run busybox cat /etc/passwd | grep -q root")

	logDone("run - redirect stdout")
}

// Regression test for https://github.com/docker/docker/issues/8259
func TestRunReuseBindVolumeThatIsSymlink(t *testing.T) {
	tmpDir, err := ioutil.TempDir(os.TempDir(), "testlink")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	linkPath := os.TempDir() + "/testlink2"
	if err := os.Symlink(tmpDir, linkPath); err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(linkPath)

	// Create first container
	cmd := exec.Command(dockerBinary, "run", "-v", fmt.Sprintf("%s:/tmp/test", linkPath), "busybox", "ls", "-lh", "/tmp/test")
	if _, err := runCommand(cmd); err != nil {
		t.Fatal(err)
	}

	// Create second container with same symlinked path
	// This will fail if the referenced issue is hit with a "Volume exists" error
	cmd = exec.Command(dockerBinary, "run", "-v", fmt.Sprintf("%s:/tmp/test", linkPath), "busybox", "ls", "-lh", "/tmp/test")
	if out, _, err := runCommandWithOutput(cmd); err != nil {
		t.Fatal(err, out)
	}

	deleteAllContainers()
	logDone("run - can remount old bindmount volume")
}

func TestVolumesNoCopyData(t *testing.T) {
	defer deleteImages("dataimage")
	defer deleteAllContainers()
	if _, err := buildImage("dataimage",
		`FROM busybox
		 RUN mkdir -p /foo
		 RUN touch /foo/bar`,
		true); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(dockerBinary, "run", "--name", "test", "-v", "/foo", "busybox")
	if _, err := runCommand(cmd); err != nil {
		t.Fatal(err)
	}

	cmd = exec.Command(dockerBinary, "run", "--volumes-from", "test", "dataimage", "ls", "-lh", "/foo/bar")
	if out, _, err := runCommandWithOutput(cmd); err == nil || !strings.Contains(out, "No such file or directory") {
		t.Fatalf("Data was copied on volumes-from but shouldn't be:\n%q", out)
	}

	tmpDir, err := ioutil.TempDir("", "docker_test_bind_mount_copy_data")
	if err != nil {
		t.Fatal(err)
	}

	defer os.RemoveAll(tmpDir)

	cmd = exec.Command(dockerBinary, "run", "-v", tmpDir+":/foo", "dataimage", "ls", "-lh", "/foo/bar")
	if out, _, err := runCommandWithOutput(cmd); err == nil || !strings.Contains(out, "No such file or directory") {
		t.Fatalf("Data was copied on bind-mount but shouldn't be:\n%q", out)
	}

	logDone("run - volumes do not copy data for volumes-from and bindmounts")
}

func TestRunVolumesNotRecreatedOnStart(t *testing.T) {
	// Clear out any remnants from other tests
	deleteAllContainers()
	info, err := ioutil.ReadDir(volumesConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(info) > 0 {
		for _, f := range info {
			if err := os.RemoveAll(volumesConfigPath + "/" + f.Name()); err != nil {
				t.Fatal(err)
			}
		}
	}

	defer deleteAllContainers()
	cmd := exec.Command(dockerBinary, "run", "-v", "/foo", "--name", "lone_starr", "busybox")
	if _, err := runCommand(cmd); err != nil {
		t.Fatal(err)
	}

	cmd = exec.Command(dockerBinary, "start", "lone_starr")
	if _, err := runCommand(cmd); err != nil {
		t.Fatal(err)
	}

	info, err = ioutil.ReadDir(volumesConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(info) != 1 {
		t.Fatalf("Expected only 1 volume have %v", len(info))
	}

	logDone("run - volumes not recreated on start")
}

func TestRunNoOutputFromPullInStdout(t *testing.T) {
	defer deleteAllContainers()
	// just run with unknown image
	cmd := exec.Command(dockerBinary, "run", "asdfsg")
	stdout := bytes.NewBuffer(nil)
	cmd.Stdout = stdout
	if err := cmd.Run(); err == nil {
		t.Fatal("Run with unknown image should fail")
	}
	if stdout.Len() != 0 {
		t.Fatalf("Stdout contains output from pull: %s", stdout)
	}
	logDone("run - no output from pull in stdout")
}

func TestRunVolumesCleanPaths(t *testing.T) {
	if _, err := buildImage("run_volumes_clean_paths",
		`FROM busybox
		 VOLUME /foo/`,
		true); err != nil {
		t.Fatal(err)
	}
	defer deleteImages("run_volumes_clean_paths")
	defer deleteAllContainers()

	cmd := exec.Command(dockerBinary, "run", "-v", "/foo", "-v", "/bar/", "--name", "dark_helmet", "run_volumes_clean_paths")
	if out, _, err := runCommandWithOutput(cmd); err != nil {
		t.Fatal(err, out)
	}

	out, err := inspectFieldMap("dark_helmet", "Volumes", "/foo/")
	if err != nil {
		t.Fatal(err)
	}
	if out != "<no value>" {
		t.Fatalf("Found unexpected volume entry for '/foo/' in volumes\n%q", out)
	}

	out, err = inspectFieldMap("dark_helmet", "Volumes", "/foo")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, volumesStoragePath) {
		t.Fatalf("Volume was not defined for /foo\n%q", out)
	}

	out, err = inspectFieldMap("dark_helmet", "Volumes", "/bar/")
	if err != nil {
		t.Fatal(err)
	}
	if out != "<no value>" {
		t.Fatalf("Found unexpected volume entry for '/bar/' in volumes\n%q", out)
	}
	out, err = inspectFieldMap("dark_helmet", "Volumes", "/bar")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, volumesStoragePath) {
		t.Fatalf("Volume was not defined for /bar\n%q", out)
	}

	logDone("run - volume paths are cleaned")
}

// Regression test for #3631
func TestRunSlowStdoutConsumer(t *testing.T) {
	defer deleteAllContainers()

	c := exec.Command("/bin/bash", "-c", dockerBinary+` run --rm -i busybox /bin/sh -c "dd if=/dev/zero of=/foo bs=1024 count=2000 &>/dev/null; catv /foo"`)

	stdout, err := c.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}

	if err := c.Start(); err != nil {
		t.Fatal(err)
	}
	n, err := consumeWithSpeed(stdout, 10000, 5*time.Millisecond, nil)
	if err != nil {
		t.Fatal(err)
	}

	expected := 2 * 1024 * 2000
	if n != expected {
		t.Fatalf("Expected %d, got %d", expected, n)
	}

	logDone("run - slow consumer")
}

func TestRunAllowPortRangeThroughExpose(t *testing.T) {
	cmd := exec.Command(dockerBinary, "run", "-d", "--expose", "3000-3003", "-P", "busybox", "top")
	out, _, err := runCommandWithOutput(cmd)
	if err != nil {
		t.Fatal(err)
	}
	id := strings.TrimSpace(out)
	portstr, err := inspectFieldJSON(id, "NetworkSettings.Ports")
	if err != nil {
		t.Fatal(err)
	}
	var ports nat.PortMap
	err = unmarshalJSON([]byte(portstr), &ports)
	for port, binding := range ports {
		portnum, _ := strconv.Atoi(strings.Split(string(port), "/")[0])
		if portnum < 3000 || portnum > 3003 {
			t.Fatalf("Port is out of range ", portnum, binding, out)
		}
		if binding == nil || len(binding) != 1 || len(binding[0].HostPort) == 0 {
			t.Fatal("Port is not mapped for the port "+port, out)
		}
	}
	if err := deleteContainer(id); err != nil {
		t.Fatal(err)
	}
	logDone("run - allow port range through --expose flag")
}

func TestRunUnknownCommand(t *testing.T) {
	defer deleteAllContainers()
	runCmd := exec.Command(dockerBinary, "create", "busybox", "/bin/nada")
	cID, _, _, err := runCommandWithStdoutStderr(runCmd)
	if err != nil {
		t.Fatalf("Failed to create container: %v, output: %q", err, cID)
	}
	cID = strings.TrimSpace(cID)

	runCmd = exec.Command(dockerBinary, "start", cID)
	_, _, _, err = runCommandWithStdoutStderr(runCmd)
	if err == nil {
		t.Fatalf("Container should not have been able to start!")
	}

	runCmd = exec.Command(dockerBinary, "inspect", "--format={{.State.ExitCode}}", cID)
	rc, _, _, err2 := runCommandWithStdoutStderr(runCmd)
	rc = strings.TrimSpace(rc)

	if err2 != nil {
		t.Fatalf("Error getting status of container: %v", err2)
	}

	if rc != "-1" {
		t.Fatalf("ExitCode(%v) was supposed to be -1", rc)
	}

	logDone("run - Unknown Command")
}

func TestRunModeIpcHost(t *testing.T) {
	hostIpc, err := os.Readlink("/proc/1/ns/ipc")
	if err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(dockerBinary, "run", "--ipc=host", "busybox", "readlink", "/proc/self/ns/ipc")
	out2, _, err := runCommandWithOutput(cmd)
	if err != nil {
		t.Fatal(err, out2)
	}

	out2 = strings.Trim(out2, "\n")
	if hostIpc != out2 {
		t.Fatalf("IPC different with --ipc=host %s != %s\n", hostIpc, out2)
	}

	cmd = exec.Command(dockerBinary, "run", "busybox", "readlink", "/proc/self/ns/ipc")
	out2, _, err = runCommandWithOutput(cmd)
	if err != nil {
		t.Fatal(err, out2)
	}

	out2 = strings.Trim(out2, "\n")
	if hostIpc == out2 {
		t.Fatalf("IPC should be different without --ipc=host %s != %s\n", hostIpc, out2)
	}
	deleteAllContainers()

	logDone("run - hostname and several network modes")
}

func TestRunModeIpcContainer(t *testing.T) {
	cmd := exec.Command(dockerBinary, "run", "-d", "busybox", "top")
	out, _, err := runCommandWithOutput(cmd)
	if err != nil {
		t.Fatal(err, out)
	}
	id := strings.TrimSpace(out)
	state, err := inspectField(id, "State.Running")
	if err != nil {
		t.Fatal(err)
	}
	if state != "true" {
		t.Fatal("Container state is 'not running'")
	}
	pid1, err := inspectField(id, "State.Pid")
	if err != nil {
		t.Fatal(err)
	}

	parentContainerIpc, err := os.Readlink(fmt.Sprintf("/proc/%s/ns/ipc", pid1))
	if err != nil {
		t.Fatal(err)
	}
	cmd = exec.Command(dockerBinary, "run", fmt.Sprintf("--ipc=container:%s", id), "busybox", "readlink", "/proc/self/ns/ipc")
	out2, _, err := runCommandWithOutput(cmd)
	if err != nil {
		t.Fatal(err, out2)
	}

	out2 = strings.Trim(out2, "\n")
	if parentContainerIpc != out2 {
		t.Fatalf("IPC different with --ipc=container:%s %s != %s\n", id, parentContainerIpc, out2)
	}
	deleteAllContainers()

	logDone("run - hostname and several network modes")
}

func TestContainerNetworkMode(t *testing.T) {
	cmd := exec.Command(dockerBinary, "run", "-d", "busybox", "top")
	out, _, err := runCommandWithOutput(cmd)
	if err != nil {
		t.Fatal(err, out)
	}
	id := strings.TrimSpace(out)
	if err := waitRun(id); err != nil {
		t.Fatal(err)
	}
	pid1, err := inspectField(id, "State.Pid")
	if err != nil {
		t.Fatal(err)
	}

	parentContainerNet, err := os.Readlink(fmt.Sprintf("/proc/%s/ns/net", pid1))
	if err != nil {
		t.Fatal(err)
	}
	cmd = exec.Command(dockerBinary, "run", fmt.Sprintf("--net=container:%s", id), "busybox", "readlink", "/proc/self/ns/net")
	out2, _, err := runCommandWithOutput(cmd)
	if err != nil {
		t.Fatal(err, out2)
	}

	out2 = strings.Trim(out2, "\n")
	if parentContainerNet != out2 {
		t.Fatalf("NET different with --net=container:%s %s != %s\n", id, parentContainerNet, out2)
	}
	deleteAllContainers()

	logDone("run - container shared network namespace")
}

func TestRunTLSverify(t *testing.T) {
	cmd := exec.Command(dockerBinary, "ps")
	out, ec, err := runCommandWithOutput(cmd)
	if err != nil || ec != 0 {
		t.Fatalf("Should have worked: %v:\n%v", err, out)
	}

	// Regardless of whether we specify true or false we need to
	// test to make sure tls is turned on if --tlsverify is specified at all

	cmd = exec.Command(dockerBinary, "--tlsverify=false", "ps")
	out, ec, err = runCommandWithOutput(cmd)
	if err == nil || ec == 0 || !strings.Contains(out, "trying to connect") {
		t.Fatalf("Should have failed: \nec:%v\nout:%v\nerr:%v", ec, out, err)
	}

	cmd = exec.Command(dockerBinary, "--tlsverify=true", "ps")
	out, ec, err = runCommandWithOutput(cmd)
	if err == nil || ec == 0 || !strings.Contains(out, "cert") {
		t.Fatalf("Should have failed: \nec:%v\nout:%v\nerr:%v", ec, out, err)
	}

	logDone("run - verify tls is set for --tlsverify")
}

func TestRunPortFromDockerRangeInUse(t *testing.T) {
	defer deleteAllContainers()
	// first find allocator current position
	cmd := exec.Command(dockerBinary, "run", "-d", "-p", ":80", "busybox", "top")
	out, _, err := runCommandWithOutput(cmd)
	if err != nil {
		t.Fatal(out, err)
	}
	id := strings.TrimSpace(out)
	cmd = exec.Command(dockerBinary, "port", id)
	out, _, err = runCommandWithOutput(cmd)
	if err != nil {
		t.Fatal(out, err)
	}
	out = strings.TrimSpace(out)
	out = strings.Split(out, ":")[1]
	lastPort, err := strconv.Atoi(out)
	if err != nil {
		t.Fatal(err)
	}
	port := lastPort + 1
	l, err := net.Listen("tcp", ":"+strconv.Itoa(port))
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	cmd = exec.Command(dockerBinary, "run", "-d", "-p", ":80", "busybox", "top")
	out, _, err = runCommandWithOutput(cmd)
	if err != nil {
		t.Fatalf(out, err)
	}
	id = strings.TrimSpace(out)
	cmd = exec.Command(dockerBinary, "port", id)
	out, _, err = runCommandWithOutput(cmd)
	if err != nil {
		t.Fatal(out, err)
	}

	logDone("run - find another port if port from autorange already bound")
}
