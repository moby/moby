package main

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"testing"

	"github.com/dotcloud/docker/daemon"
	"github.com/dotcloud/docker/runconfig"
)

// Make sure we can create a simple container with some args
func TestDockerCreateArgs(t *testing.T) {
	runCmd := exec.Command(dockerBinary, "create", "busybox", "command", "arg1", "arg2", "arg with space")
	out, _, _, err := runCommandWithStdoutStderr(runCmd)
	errorOut(err, t, out)

	cleanedContainerID := stripTrailingCharacters(out)

	inspectCmd := exec.Command(dockerBinary, "inspect", cleanedContainerID)
	inspectOut, _, err := runCommandWithOutput(inspectCmd)
	errorOut(err, t, fmt.Sprintf("out should've been a container id: %v %v", inspectOut, err))

	containers := []*daemon.Container{}
	if err := json.Unmarshal([]byte(inspectOut), &containers); err != nil {
		t.Fatalf("Error inspecting the container: %s", err)
	}
	if len(containers) != 1 {
		t.Fatalf("Unepexted container count. Expected 0, recieved: %d", len(containers))
	}

	c := containers[0]
	if c.Path != "command" {
		t.Fatalf("Unepexted container path. Expected command, recieved: %s", c.Path)
	}

	if len(c.Args) != 3 ||
		c.Args[0] != "arg1" ||
		c.Args[1] != "arg2" ||
		c.Args[2] != "arg with space" {
		t.Fatalf("Unepexted args. Expected recieved: %v", c.Args)
	}

	deleteAllContainers()

	logDone("create - args")
}

type HostConfData struct {
	HostConfig *runconfig.HostConfig
}

// Make sure we can set hostconfig options too
func TestDockerCreateHostConfig(t *testing.T) {
	runCmd := exec.Command(dockerBinary, "create", "-P", "busybox", "echo")
	out, _, _, err := runCommandWithStdoutStderr(runCmd)
	errorOut(err, t, out)

	cleanedContainerID := stripTrailingCharacters(out)

	inspectCmd := exec.Command(dockerBinary, "inspect", cleanedContainerID)
	inspectOut, _, err := runCommandWithOutput(inspectCmd)
	errorOut(err, t, fmt.Sprintf("out should've been a container id: %v %v", inspectOut, err))

	containers := []*HostConfData{}
	if err := json.Unmarshal([]byte(inspectOut), &containers); err != nil {
		t.Fatalf("Error inspecting the container: %s", err)
	}
	if len(containers) != 1 {
		t.Fatalf("Unepexted container count. Expected 0, recieved: %d", len(containers))
	}

	c := containers[0]
	if c.HostConfig == nil {
		t.Fatalf("Expected HostConfig, got none")
	}

	if !c.HostConfig.PublishAllPorts {
		t.Fatalf("Expected PublishAllPorts, got false")
	}

	deleteAllContainers()

	logDone("create - hostconfig")
}

// "test123" should be printed by docker create + start
func TestDockerCreateEchoStdout(t *testing.T) {
	runCmd := exec.Command(dockerBinary, "create", "busybox", "echo", "test123")
	out, _, _, err := runCommandWithStdoutStderr(runCmd)
	errorOut(err, t, out)

	cleanedContainerID := stripTrailingCharacters(out)

	runCmd = exec.Command(dockerBinary, "start", "-ai", cleanedContainerID)
	out, _, _, err = runCommandWithStdoutStderr(runCmd)
	errorOut(err, t, out)

	if out != "test123\n" {
		t.Errorf("container should've printed 'test123', got '%s'", out)
	}

	deleteAllContainers()

	logDone("create - echo test123")
}
