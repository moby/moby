package main

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/docker/docker/pkg/mount"
	"github.com/docker/docker/pkg/networkfs/resolvconf"
)

// "test123" should be printed by docker run
func TestDockerRunEchoStdout(t *testing.T) {
	runCmd := exec.Command(dockerBinary, "run", "busybox", "echo", "test123")
	out, _, _, err := runCommandWithStdoutStderr(runCmd)
	errorOut(err, t, out)

	if out != "test123\n" {
		t.Errorf("container should've printed 'test123'")
	}

	deleteAllContainers()

	logDone("run - echo test123")
}

// "test" should be printed
func TestDockerRunEchoStdoutWithMemoryLimit(t *testing.T) {
	runCmd := exec.Command(dockerBinary, "run", "-m", "2786432", "busybox", "echo", "test")
	out, _, _, err := runCommandWithStdoutStderr(runCmd)
	errorOut(err, t, out)

	if out != "test\n" {
		t.Errorf("container should've printed 'test'")

	}

	deleteAllContainers()

	logDone("run - echo with memory limit")
}

// "test" should be printed
func TestDockerRunEchoStdoutWitCPULimit(t *testing.T) {
	runCmd := exec.Command(dockerBinary, "run", "-c", "1000", "busybox", "echo", "test")
	out, _, _, err := runCommandWithStdoutStderr(runCmd)
	errorOut(err, t, out)

	if out != "test\n" {
		t.Errorf("container should've printed 'test'")
	}

	deleteAllContainers()

	logDone("run - echo with CPU limit")
}

// "test" should be printed
func TestDockerRunEchoStdoutWithCPUAndMemoryLimit(t *testing.T) {
	runCmd := exec.Command(dockerBinary, "run", "-c", "1000", "-m", "2786432", "busybox", "echo", "test")
	out, _, _, err := runCommandWithStdoutStderr(runCmd)
	errorOut(err, t, out)

	if out != "test\n" {
		t.Errorf("container should've printed 'test'")
	}

	deleteAllContainers()

	logDone("run - echo with CPU and memory limit")
}

// "test" should be printed
func TestDockerRunEchoNamedContainer(t *testing.T) {
	runCmd := exec.Command(dockerBinary, "run", "--name", "testfoonamedcontainer", "busybox", "echo", "test")
	out, _, _, err := runCommandWithStdoutStderr(runCmd)
	errorOut(err, t, out)

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
func TestDockerRunLeakyFileDescriptors(t *testing.T) {
	runCmd := exec.Command(dockerBinary, "run", "busybox", "ls", "-C", "/proc/self/fd")
	out, _, _, err := runCommandWithStdoutStderr(runCmd)
	errorOut(err, t, out)

	// normally, we should only get 0, 1, and 2, but 3 gets created by "ls" when it does "opendir" on the "fd" directory
	if out != "0  1  2  3\n" {
		t.Errorf("container should've printed '0  1  2  3', not: %s", out)
	}

	deleteAllContainers()

	logDone("run - check file descriptor leakage")
}

// it should be possible to ping Google DNS resolver
// this will fail when Internet access is unavailable
func TestDockerRunPingGoogle(t *testing.T) {
	runCmd := exec.Command(dockerBinary, "run", "busybox", "ping", "-c", "1", "8.8.8.8")
	out, _, _, err := runCommandWithStdoutStderr(runCmd)
	errorOut(err, t, out)

	errorOut(err, t, "container should've been able to ping 8.8.8.8")

	deleteAllContainers()

	logDone("run - ping 8.8.8.8")
}

// the exit code should be 0
// some versions of lxc might make this test fail
func TestDockerRunExitCodeZero(t *testing.T) {
	runCmd := exec.Command(dockerBinary, "run", "busybox", "true")
	exitCode, err := runCommand(runCmd)
	errorOut(err, t, fmt.Sprintf("%s", err))

	if exitCode != 0 {
		t.Errorf("container should've exited with exit code 0")
	}

	deleteAllContainers()

	logDone("run - exit with 0")
}

// the exit code should be 1
// some versions of lxc might make this test fail
func TestDockerRunExitCodeOne(t *testing.T) {
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
	errorOut(err, t, out)

	out = stripTrailingCharacters(out)

	inspectCmd := exec.Command(dockerBinary, "inspect", out)
	inspectOut, _, err := runCommandWithOutput(inspectCmd)
	errorOut(err, t, fmt.Sprintf("out should've been a container id: %s %s", out, inspectOut))

	waitCmd := exec.Command(dockerBinary, "wait", out)
	_, _, err = runCommandWithOutput(waitCmd)
	errorOut(err, t, fmt.Sprintf("error thrown while waiting for container: %s", out))

	logsCmd := exec.Command(dockerBinary, "logs", out)
	containerLogs, _, err := runCommandWithOutput(logsCmd)
	errorOut(err, t, fmt.Sprintf("error thrown while trying to get container logs: %s", err))

	containerLogs = stripTrailingCharacters(containerLogs)

	if containerLogs != "blahblah" {
		t.Errorf("logs didn't print the container's logs %s", containerLogs)
	}

	rmCmd := exec.Command(dockerBinary, "rm", out)
	_, _, err = runCommandWithOutput(rmCmd)
	errorOut(err, t, fmt.Sprintf("rm failed to remove container %s", err))

	deleteAllContainers()

	logDone("run - pipe in with -i -a stdin")
}

// the container's ID should be printed when starting a container in detached mode
func TestDockerRunDetachedContainerIDPrinting(t *testing.T) {
	runCmd := exec.Command(dockerBinary, "run", "-d", "busybox", "true")
	out, _, _, err := runCommandWithStdoutStderr(runCmd)
	errorOut(err, t, out)

	out = stripTrailingCharacters(out)

	inspectCmd := exec.Command(dockerBinary, "inspect", out)
	inspectOut, _, err := runCommandWithOutput(inspectCmd)
	errorOut(err, t, fmt.Sprintf("out should've been a container id: %s %s", out, inspectOut))

	waitCmd := exec.Command(dockerBinary, "wait", out)
	_, _, err = runCommandWithOutput(waitCmd)
	errorOut(err, t, fmt.Sprintf("error thrown while waiting for container: %s", out))

	rmCmd := exec.Command(dockerBinary, "rm", out)
	rmOut, _, err := runCommandWithOutput(rmCmd)
	errorOut(err, t, "rm failed to remove container")

	rmOut = stripTrailingCharacters(rmOut)
	if rmOut != out {
		t.Errorf("rm didn't print the container ID %s %s", out, rmOut)
	}

	deleteAllContainers()

	logDone("run - print container ID in detached mode")
}

// the working directory should be set correctly
func TestDockerRunWorkingDirectory(t *testing.T) {
	runCmd := exec.Command(dockerBinary, "run", "-w", "/root", "busybox", "pwd")
	out, _, _, err := runCommandWithStdoutStderr(runCmd)
	errorOut(err, t, out)

	out = stripTrailingCharacters(out)

	if out != "/root" {
		t.Errorf("-w failed to set working directory")
	}

	runCmd = exec.Command(dockerBinary, "run", "--workdir", "/root", "busybox", "pwd")
	out, _, _, err = runCommandWithStdoutStderr(runCmd)
	errorOut(err, t, out)

	out = stripTrailingCharacters(out)

	if out != "/root" {
		t.Errorf("--workdir failed to set working directory")
	}

	deleteAllContainers()

	logDone("run - run with working directory set by -w")
	logDone("run - run with working directory set by --workdir")
}

// pinging Google's DNS resolver should fail when we disable the networking
func TestDockerRunWithoutNetworking(t *testing.T) {
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
func TestDockerRunWithVolumesAsFiles(t *testing.T) {
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
func TestDockerRunWithVolumesFromExited(t *testing.T) {
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
func TestDockerRunWithRelativePath(t *testing.T) {
	runCmd := exec.Command(dockerBinary, "run", "-v", "tmp:/other-tmp", "busybox", "true")
	if _, _, _, err := runCommandWithStdoutStderr(runCmd); err == nil {
		t.Fatalf("relative path should result in an error")
	}

	deleteAllContainers()

	logDone("run - volume with relative path")
}

func TestVolumesMountedAsReadonly(t *testing.T) {
	cmd := exec.Command(dockerBinary, "run", "-v", "/test:/test:ro", "busybox", "touch", "/test/somefile")
	if code, err := runCommand(cmd); err == nil || code == 0 {
		t.Fatalf("run should fail because volume is ro: exit code %d", code)
	}

	deleteAllContainers()

	logDone("run - volumes as readonly mount")
}

func TestVolumesFromInReadonlyMode(t *testing.T) {
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
func TestVolumesFromInReadWriteMode(t *testing.T) {
	cmd := exec.Command(dockerBinary, "run", "--name", "parent", "-v", "/test", "busybox", "true")
	if _, err := runCommand(cmd); err != nil {
		t.Fatal(err)
	}

	cmd = exec.Command(dockerBinary, "run", "--volumes-from", "parent", "busybox", "touch", "/test/file")
	if _, err := runCommand(cmd); err != nil {
		t.Fatal(err)
	}

	deleteAllContainers()

	logDone("run - volumes from as read write mount")
}

// Test for #1351
func TestApplyVolumesFromBeforeVolumes(t *testing.T) {
	cmd := exec.Command(dockerBinary, "run", "--name", "parent", "-v", "/test", "busybox", "touch", "/test/foo")
	if _, err := runCommand(cmd); err != nil {
		t.Fatal(err)
	}

	cmd = exec.Command(dockerBinary, "run", "--volumes-from", "parent", "-v", "/test", "busybox", "cat", "/test/foo")
	if _, err := runCommand(cmd); err != nil {
		t.Fatal(err)
	}

	deleteAllContainers()

	logDone("run - volumes from mounted first")
}

func TestMultipleVolumesFrom(t *testing.T) {
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
func TestVerifyContainerID(t *testing.T) {
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
func TestCreateVolume(t *testing.T) {
	cmd := exec.Command(dockerBinary, "run", "-v", "/var/lib/data", "busybox", "true")
	if _, err := runCommand(cmd); err != nil {
		t.Fatal(err)
	}

	deleteAllContainers()

	logDone("run - create docker managed volume")
}

// Test that creating a volume with a symlink in its path works correctly. Test for #5152.
// Note that this bug happens only with symlinks with a target that starts with '/'.
func TestCreateVolumeWithSymlink(t *testing.T) {
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
func TestVolumesFromSymlinkPath(t *testing.T) {
	buildCmd := exec.Command(dockerBinary, "build", "-t", "docker-test-volumesfromsymlinkpath", "-")
	buildCmd.Stdin = strings.NewReader(`FROM busybox
		RUN mkdir /baz && ln -s /baz /foo
		VOLUME ["/foo/bar"]`)
	buildCmd.Dir = workingDirectory
	err := buildCmd.Run()
	if err != nil {
		t.Fatalf("could not build 'docker-test-volumesfromsymlinkpath': %v", err)
	}

	cmd := exec.Command(dockerBinary, "run", "--name", "test-volumesfromsymlinkpath", "docker-test-volumesfromsymlinkpath")
	exitCode, err := runCommand(cmd)
	if err != nil || exitCode != 0 {
		t.Fatalf("[run] (volume) err: %v, exitcode: %d", err, exitCode)
	}

	cmd = exec.Command(dockerBinary, "run", "--volumes-from", "test-volumesfromsymlinkpath", "busybox", "sh", "-c", "ls /foo | grep -q bar")
	exitCode, err = runCommand(cmd)
	if err != nil || exitCode != 0 {
		t.Fatalf("[run] err: %v, exitcode: %d", err, exitCode)
	}

	deleteImages("docker-test-volumesfromsymlinkpath")
	deleteAllContainers()

	logDone("run - volumes-from symlink path")
}

func TestExitCode(t *testing.T) {
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

func TestUserDefaultsToRoot(t *testing.T) {
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

func TestUserByName(t *testing.T) {
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

func TestUserByID(t *testing.T) {
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

func TestUserByIDBig(t *testing.T) {
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

func TestUserByIDNegative(t *testing.T) {
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

func TestUserByIDZero(t *testing.T) {
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

func TestUserNotFound(t *testing.T) {
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

func TestEnvironment(t *testing.T) {
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
		t.Fatalf("Wrong environment: should be %d variables, not: '%s'\n", len(goodEnv), strings.Join(actualEnv, ", "))
	}
	for i := range goodEnv {
		if actualEnv[i] != goodEnv[i] {
			t.Fatalf("Wrong environment variable: should be %s, not %s", goodEnv[i], actualEnv[i])
		}
	}

	deleteAllContainers()

	logDone("run - verify environment")
}

func TestContainerNetwork(t *testing.T) {
	cmd := exec.Command(dockerBinary, "run", "busybox", "ping", "-c", "1", "127.0.0.1")
	if _, err := runCommand(cmd); err != nil {
		t.Fatal(err)
	}

	deleteAllContainers()

	logDone("run - test container network via ping")
}

// Issue #4681
func TestLoopbackWhenNetworkDisabled(t *testing.T) {
	cmd := exec.Command(dockerBinary, "run", "--net=none", "busybox", "ping", "-c", "1", "127.0.0.1")
	if _, err := runCommand(cmd); err != nil {
		t.Fatal(err)
	}

	deleteAllContainers()

	logDone("run - test container loopback when networking disabled")
}

func TestNetHostNotAllowedWithLinks(t *testing.T) {
	_, _, err := cmd(t, "run", "--name", "linked", "busybox", "true")

	cmd := exec.Command(dockerBinary, "run", "--net=host", "--link", "linked:linked", "busybox", "true")
	_, _, err = runCommandWithOutput(cmd)
	if err == nil {
		t.Fatal("Expected error")
	}

	deleteAllContainers()

	logDone("run - don't allow --net=host to be used with links")
}

func TestLoopbackOnlyExistsWhenNetworkingDisabled(t *testing.T) {
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

func TestPrivilegedCanMknod(t *testing.T) {
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

func TestUnPrivilegedCanMknod(t *testing.T) {
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

func TestCapDropInvalid(t *testing.T) {
	cmd := exec.Command(dockerBinary, "run", "--cap-drop=CHPASS", "busybox", "ls")
	out, _, err := runCommandWithOutput(cmd)
	if err == nil {
		t.Fatal(err, out)
	}

	logDone("run - test --cap-drop=CHPASS invalid")
}

func TestCapDropCannotMknod(t *testing.T) {
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

func TestCapDropCannotMknodLowerCase(t *testing.T) {
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

func TestCapDropALLCannotMknod(t *testing.T) {
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

func TestCapDropALLAddMknodCannotMknod(t *testing.T) {
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

func TestCapAddInvalid(t *testing.T) {
	cmd := exec.Command(dockerBinary, "run", "--cap-add=CHPASS", "busybox", "ls")
	out, _, err := runCommandWithOutput(cmd)
	if err == nil {
		t.Fatal(err, out)
	}

	logDone("run - test --cap-add=CHPASS invalid")
}

func TestCapAddCanDownInterface(t *testing.T) {
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

func TestCapAddALLCanDownInterface(t *testing.T) {
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

func TestCapAddALLDropNetAdminCanDownInterface(t *testing.T) {
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

func TestPrivilegedCanMount(t *testing.T) {
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

func TestUnPrivilegedCannotMount(t *testing.T) {
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

func TestSysNotWritableInNonPrivilegedContainers(t *testing.T) {
	cmd := exec.Command(dockerBinary, "run", "busybox", "touch", "/sys/kernel/profiling")
	if code, err := runCommand(cmd); err == nil || code == 0 {
		t.Fatal("sys should not be writable in a non privileged container")
	}

	deleteAllContainers()

	logDone("run - sys not writable in non privileged container")
}

func TestSysWritableInPrivilegedContainers(t *testing.T) {
	cmd := exec.Command(dockerBinary, "run", "--privileged", "busybox", "touch", "/sys/kernel/profiling")
	if code, err := runCommand(cmd); err != nil || code != 0 {
		t.Fatalf("sys should be writable in privileged container")
	}

	deleteAllContainers()

	logDone("run - sys writable in privileged container")
}

func TestProcNotWritableInNonPrivilegedContainers(t *testing.T) {
	cmd := exec.Command(dockerBinary, "run", "busybox", "touch", "/proc/sysrq-trigger")
	if code, err := runCommand(cmd); err == nil || code == 0 {
		t.Fatal("proc should not be writable in a non privileged container")
	}

	deleteAllContainers()

	logDone("run - proc not writable in non privileged container")
}

func TestProcWritableInPrivilegedContainers(t *testing.T) {
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

func TestDeviceNumbers(t *testing.T) {
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

func TestThatCharacterDevicesActLikeCharacterDevices(t *testing.T) {
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

func TestAddingOptionalDevices(t *testing.T) {
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

func TestModeHostname(t *testing.T) {
	cmd := exec.Command(dockerBinary, "run", "-h=testhostname", "busybox", "cat", "/etc/hostname")

	out, _, err := runCommandWithOutput(cmd)
	if err != nil {
		t.Fatal(err, out)
	}

	if actual := strings.Trim(out, "\r\n"); actual != "testhostname" {
		t.Fatalf("expected 'testhostname', but says: '%s'", actual)
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
		t.Fatalf("expected '%s', but says: '%s'", hostname, actual)
	}

	deleteAllContainers()

	logDone("run - hostname and several network modes")
}

func TestRootWorkdir(t *testing.T) {
	s, _, err := cmd(t, "run", "--workdir", "/", "busybox", "pwd")
	if err != nil {
		t.Fatal(s, err)
	}
	if s != "/\n" {
		t.Fatalf("pwd returned '%s' (expected /\\n)", s)
	}

	deleteAllContainers()

	logDone("run - workdir /")
}

func TestAllowBindMountingRoot(t *testing.T) {
	s, _, err := cmd(t, "run", "-v", "/:/host", "busybox", "ls", "/host")
	if err != nil {
		t.Fatal(s, err)
	}

	deleteAllContainers()

	logDone("run - bind mount / as volume")
}

func TestDisallowBindMountingRootToRoot(t *testing.T) {
	cmd := exec.Command(dockerBinary, "run", "-v", "/:/", "busybox", "ls", "/host")
	out, _, err := runCommandWithOutput(cmd)
	if err == nil {
		t.Fatal(out, err)
	}

	deleteAllContainers()

	logDone("run - bind mount /:/ as volume should fail")
}

// Test recursive bind mount works by default
func TestDockerRunWithVolumesIsRecursive(t *testing.T) {
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

	logDone("run - volumes are bind mounted recuursively")
}

func TestDnsDefaultOptions(t *testing.T) {
	cmd := exec.Command(dockerBinary, "run", "busybox", "cat", "/etc/resolv.conf")

	actual, _, err := runCommandWithOutput(cmd)
	if err != nil {
		t.Fatal(err, actual)
	}

	resolvConf, err := ioutil.ReadFile("/etc/resolv.conf")
	if os.IsNotExist(err) {
		t.Fatalf("/etc/resolv.conf does not exist")
	}

	if actual != string(resolvConf) {
		t.Fatalf("expected resolv.conf is not the same of actual")
	}

	deleteAllContainers()

	logDone("run - dns default options")
}

func TestDnsOptions(t *testing.T) {
	cmd := exec.Command(dockerBinary, "run", "--dns=127.0.0.1", "--dns-search=mydomain", "busybox", "cat", "/etc/resolv.conf")

	out, _, err := runCommandWithOutput(cmd)
	if err != nil {
		t.Fatal(err, out)
	}

	actual := strings.Replace(strings.Trim(out, "\r\n"), "\n", " ", -1)
	if actual != "nameserver 127.0.0.1 search mydomain" {
		t.Fatalf("expected 'nameserver 127.0.0.1 search mydomain', but says: '%s'", actual)
	}

	cmd = exec.Command(dockerBinary, "run", "--dns=127.0.0.1", "--dns-search=.", "busybox", "cat", "/etc/resolv.conf")

	out, _, err = runCommandWithOutput(cmd)
	if err != nil {
		t.Fatal(err, out)
	}

	actual = strings.Replace(strings.Trim(strings.Trim(out, "\r\n"), " "), "\n", " ", -1)
	if actual != "nameserver 127.0.0.1" {
		t.Fatalf("expected 'nameserver 127.0.0.1', but says: '%s'", actual)
	}

	logDone("run - dns options")
}

func TestDnsOptionsBasedOnHostResolvConf(t *testing.T) {
	resolvConf, err := ioutil.ReadFile("/etc/resolv.conf")
	if os.IsNotExist(err) {
		t.Fatalf("/etc/resolv.conf does not exist")
	}

	hostNamservers := resolvconf.GetNameservers(resolvConf)
	hostSearch := resolvconf.GetSearchDomains(resolvConf)

	cmd := exec.Command(dockerBinary, "run", "--dns=127.0.0.1", "busybox", "cat", "/etc/resolv.conf")

	out, _, err := runCommandWithOutput(cmd)
	if err != nil {
		t.Fatal(err, out)
	}

	if actualNameservers := resolvconf.GetNameservers([]byte(out)); string(actualNameservers[0]) != "127.0.0.1" {
		t.Fatalf("expected '127.0.0.1', but says: '%s'", string(actualNameservers[0]))
	}

	actualSearch := resolvconf.GetSearchDomains([]byte(out))
	if len(actualSearch) != len(hostSearch) {
		t.Fatalf("expected '%s' search domain(s), but it has: '%s'", len(hostSearch), len(actualSearch))
	}
	for i := range actualSearch {
		if actualSearch[i] != hostSearch[i] {
			t.Fatalf("expected '%s' domain, but says: '%s'", actualSearch[i], hostSearch[i])
		}
	}

	cmd = exec.Command(dockerBinary, "run", "--dns-search=mydomain", "busybox", "cat", "/etc/resolv.conf")

	out, _, err = runCommandWithOutput(cmd)
	if err != nil {
		t.Fatal(err, out)
	}

	actualNameservers := resolvconf.GetNameservers([]byte(out))
	if len(actualNameservers) != len(hostNamservers) {
		t.Fatalf("expected '%s' nameserver(s), but it has: '%s'", len(hostNamservers), len(actualNameservers))
	}
	for i := range actualNameservers {
		if actualNameservers[i] != hostNamservers[i] {
			t.Fatalf("expected '%s' nameserver, but says: '%s'", actualNameservers[i], hostNamservers[i])
		}
	}

	if actualSearch = resolvconf.GetSearchDomains([]byte(out)); string(actualSearch[0]) != "mydomain" {
		t.Fatalf("expected 'mydomain', but says: '%s'", string(actualSearch[0]))
	}

	deleteAllContainers()

	logDone("run - dns options based on host resolv.conf")
}

// Regression test for #6983
func TestAttachStdErrOnlyTTYMode(t *testing.T) {
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
func TestAttachStdOutOnlyTTYMode(t *testing.T) {
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
func TestAttachStdOutAndErrTTYMode(t *testing.T) {
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

func TestState(t *testing.T) {
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
func TestCopyVolumeUidGid(t *testing.T) {
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
func TestCopyVolumeContent(t *testing.T) {
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
	cmd := exec.Command(dockerBinary, "run", "--rm", "-v", "/hello", name, "sh", "-c", "find", "/hello")
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
func TestWriteHostsFileAndNotCommit(t *testing.T) {
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
func TestWriteHostnameFileAndNotCommit(t *testing.T) {
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
func TestWriteResolvFileAndNotCommit(t *testing.T) {
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
	name := "baddevice"
	cmd := exec.Command(dockerBinary, "run", "--name", name, "--device", "/etc", "busybox", "true")
	out, _, err := runCommandWithOutput(cmd)
	if err == nil {
		t.Fatal("Run should fail with bad device")
	}
	expected := `"/etc": not a device node`
	if !strings.Contains(out, expected) {
		t.Fatalf("Output should contain %q, actual out: %q", expected, out)
	}
	logDone("run - error with bad device")
}
