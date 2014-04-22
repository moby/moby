package main

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strings"
	"sync"
	"testing"
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
	runCmd := exec.Command(dockerBinary, "run", "--networking=false", "busybox", "ping", "-c", "1", "8.8.8.8")
	out, _, exitCode, err := runCommandWithStdoutStderr(runCmd)
	if err != nil && exitCode != 1 {
		t.Fatal(out, err)
	}
	if exitCode != 1 {
		t.Errorf("--networking=false should've disabled the network; the container shouldn't have been able to ping 8.8.8.8")
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

	logDone("run - disable networking with --networking=false")
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

	logDone("run - create docker mangaed volume")
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
	cmd := exec.Command(dockerBinary, "run", "-h", "testing", "-e=FALSE=true", "-e=TRUE", "-e=TRICKY", "busybox", "env")
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
		"HOME=/",
		"HOSTNAME=testing",
		"FALSE=true",
		"TRUE=false",
		"TRICKY=tri",
		"cky",
		"",
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
	cmd := exec.Command(dockerBinary, "run", "--networking=false", "busybox", "ping", "-c", "1", "127.0.0.1")
	if _, err := runCommand(cmd); err != nil {
		t.Fatal(err)
	}

	deleteAllContainers()

	logDone("run - test container loopback when networking disabled")
}

func TestLoopbackOnlyExistsWhenNetworkingDisabled(t *testing.T) {
	cmd := exec.Command(dockerBinary, "run", "--networking=false", "busybox", "ip", "a", "show", "up")
	out, _, err := runCommandWithOutput(cmd)
	if err != nil {
		t.Fatal(err, out)
	}

	interfaces := regexp.MustCompile(`(?m)^[0-9]+: [a-zA-Z0-9]+`).FindAllString(out, -1)
	if len(interfaces) != 1 {
		t.Fatalf("Wrong interface count in test container: expected [*: lo], got %s", interfaces)
	}
	if !strings.HasSuffix(interfaces[0], ": lo") {
		t.Fatalf("Wrong interface in test container: expected [*: lo], got %s", interfaces)
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
