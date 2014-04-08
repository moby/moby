package main

import (
	"fmt"
	"os/exec"
	"strings"
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
