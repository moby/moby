package main

import (
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestRestartStoppedContainer(t *testing.T) {
	defer deleteAllContainers()

	runCmd := exec.Command(dockerBinary, "run", "-d", "busybox", "echo", "foobar")
	out, _, err := runCommandWithOutput(runCmd)
	if err != nil {
		t.Fatal(out, err)
	}

	cleanedContainerID := stripTrailingCharacters(out)

	runCmd = exec.Command(dockerBinary, "wait", cleanedContainerID)
	if out, _, err = runCommandWithOutput(runCmd); err != nil {
		t.Fatal(out, err)
	}

	runCmd = exec.Command(dockerBinary, "logs", cleanedContainerID)
	out, _, err = runCommandWithOutput(runCmd)
	if err != nil {
		t.Fatal(out, err)
	}

	if out != "foobar\n" {
		t.Errorf("container should've printed 'foobar'")
	}

	runCmd = exec.Command(dockerBinary, "restart", cleanedContainerID)
	if out, _, err = runCommandWithOutput(runCmd); err != nil {
		t.Fatal(out, err)
	}

	runCmd = exec.Command(dockerBinary, "logs", cleanedContainerID)
	out, _, err = runCommandWithOutput(runCmd)
	if err != nil {
		t.Fatal(out, err)
	}

	if out != "foobar\nfoobar\n" {
		t.Errorf("container should've printed 'foobar' twice")
	}

	logDone("restart - echo foobar for stopped container")
}

func TestRestartRunningContainer(t *testing.T) {
	defer deleteAllContainers()

	runCmd := exec.Command(dockerBinary, "run", "-d", "busybox", "sh", "-c", "echo foobar && sleep 30 && echo 'should not print this'")
	out, _, err := runCommandWithOutput(runCmd)
	if err != nil {
		t.Fatal(out, err)
	}

	cleanedContainerID := stripTrailingCharacters(out)

	time.Sleep(1 * time.Second)

	runCmd = exec.Command(dockerBinary, "logs", cleanedContainerID)
	out, _, err = runCommandWithOutput(runCmd)
	if err != nil {
		t.Fatal(out, err)
	}

	if out != "foobar\n" {
		t.Errorf("container should've printed 'foobar'")
	}

	runCmd = exec.Command(dockerBinary, "restart", "-t", "1", cleanedContainerID)
	if out, _, err = runCommandWithOutput(runCmd); err != nil {
		t.Fatal(out, err)
	}

	runCmd = exec.Command(dockerBinary, "logs", cleanedContainerID)
	out, _, err = runCommandWithOutput(runCmd)
	if err != nil {
		t.Fatal(out, err)
	}

	time.Sleep(1 * time.Second)

	if out != "foobar\nfoobar\n" {
		t.Errorf("container should've printed 'foobar' twice")
	}

	logDone("restart - echo foobar for running container")
}

// Test that restarting a container with a volume does not create a new volume on restart. Regression test for #819.
func TestRestartWithVolumes(t *testing.T) {
	defer deleteAllContainers()

	runCmd := exec.Command(dockerBinary, "run", "-d", "-v", "/test", "busybox", "top")
	out, _, err := runCommandWithOutput(runCmd)
	if err != nil {
		t.Fatal(out, err)
	}

	cleanedContainerID := stripTrailingCharacters(out)

	runCmd = exec.Command(dockerBinary, "inspect", "--format", "{{ len .Volumes }}", cleanedContainerID)
	out, _, err = runCommandWithOutput(runCmd)
	if err != nil {
		t.Fatal(out, err)
	}

	if out = strings.Trim(out, " \n\r"); out != "1" {
		t.Errorf("expect 1 volume received %s", out)
	}

	runCmd = exec.Command(dockerBinary, "inspect", "--format", "{{ .Volumes }}", cleanedContainerID)
	volumes, _, err := runCommandWithOutput(runCmd)
	if err != nil {
		t.Fatal(volumes, err)
	}

	runCmd = exec.Command(dockerBinary, "restart", cleanedContainerID)
	if out, _, err = runCommandWithOutput(runCmd); err != nil {
		t.Fatal(out, err)
	}

	runCmd = exec.Command(dockerBinary, "inspect", "--format", "{{ len .Volumes }}", cleanedContainerID)
	out, _, err = runCommandWithOutput(runCmd)
	if err != nil {
		t.Fatal(out, err)
	}

	if out = strings.Trim(out, " \n\r"); out != "1" {
		t.Errorf("expect 1 volume after restart received %s", out)
	}

	runCmd = exec.Command(dockerBinary, "inspect", "--format", "{{ .Volumes }}", cleanedContainerID)
	volumesAfterRestart, _, err := runCommandWithOutput(runCmd)
	if err != nil {
		t.Fatal(volumesAfterRestart, err)
	}

	if volumes != volumesAfterRestart {
		volumes = strings.Trim(volumes, " \n\r")
		volumesAfterRestart = strings.Trim(volumesAfterRestart, " \n\r")
		t.Errorf("expected volume path: %s Actual path: %s", volumes, volumesAfterRestart)
	}

	logDone("restart - does not create a new volume on restart")
}

func TestRestartPolicyNO(t *testing.T) {
	defer deleteAllContainers()

	cmd := exec.Command(dockerBinary, "run", "-d", "--restart=no", "busybox", "false")
	out, _, err := runCommandWithOutput(cmd)
	if err != nil {
		t.Fatal(err, out)
	}

	id := strings.TrimSpace(string(out))
	name, err := inspectField(id, "HostConfig.RestartPolicy.Name")
	if err != nil {
		t.Fatal(err, out)
	}
	if name != "no" {
		t.Fatalf("Container restart policy name is %s, expected %s", name, "no")
	}

	logDone("restart - recording restart policy name for --restart=no")
}

func TestRestartPolicyAlways(t *testing.T) {
	defer deleteAllContainers()

	cmd := exec.Command(dockerBinary, "run", "-d", "--restart=always", "busybox", "false")
	out, _, err := runCommandWithOutput(cmd)
	if err != nil {
		t.Fatal(err, out)
	}

	id := strings.TrimSpace(string(out))
	name, err := inspectField(id, "HostConfig.RestartPolicy.Name")
	if err != nil {
		t.Fatal(err, out)
	}
	if name != "always" {
		t.Fatalf("Container restart policy name is %s, expected %s", name, "always")
	}

	logDone("restart - recording restart policy name for --restart=always")
}

func TestRestartPolicyOnFailure(t *testing.T) {
	defer deleteAllContainers()

	cmd := exec.Command(dockerBinary, "run", "-d", "--restart=on-failure:1", "busybox", "false")
	out, _, err := runCommandWithOutput(cmd)
	if err != nil {
		t.Fatal(err, out)
	}

	id := strings.TrimSpace(string(out))
	name, err := inspectField(id, "HostConfig.RestartPolicy.Name")
	if err != nil {
		t.Fatal(err, out)
	}
	if name != "on-failure" {
		t.Fatalf("Container restart policy name is %s, expected %s", name, "on-failure")
	}

	logDone("restart - recording restart policy name for --restart=on-failure")
}

// a good container with --restart=on-failure:3
// MaximumRetryCount!=0; RestartCount=0
func TestContainerRestartwithGoodContainer(t *testing.T) {
	defer deleteAllContainers()
	out, err := exec.Command(dockerBinary, "run", "-d", "--restart=on-failure:3", "busybox", "true").CombinedOutput()
	if err != nil {
		t.Fatal(string(out), err)
	}
	id := strings.TrimSpace(string(out))
	if err := waitInspect(id, "{{ .State.Restarting }} {{ .State.Running }}", "false false", 5); err != nil {
		t.Fatal(err)
	}
	count, err := inspectField(id, "RestartCount")
	if err != nil {
		t.Fatal(err)
	}
	if count != "0" {
		t.Fatalf("Container was restarted %s times, expected %d", count, 0)
	}
	MaximumRetryCount, err := inspectField(id, "HostConfig.RestartPolicy.MaximumRetryCount")
	if err != nil {
		t.Fatal(err)
	}
	if MaximumRetryCount != "3" {
		t.Fatalf("Container Maximum Retry Count is %s, expected %s", MaximumRetryCount, "3")
	}

	logDone("restart - for a good container with restart policy, MaximumRetryCount is not 0 and RestartCount is 0")
}
