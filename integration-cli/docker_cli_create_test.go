package main

import (
	"encoding/json"
	"github.com/docker/docker/nat"
	"os"
	"os/exec"
	"testing"
	"time"
)

// Make sure we can create a simple container with some args
func TestCreateArgs(t *testing.T) {
	runCmd := exec.Command(dockerBinary, "create", "busybox", "command", "arg1", "arg2", "arg with space")
	out, _, _, err := runCommandWithStdoutStderr(runCmd)
	if err != nil {
		t.Fatal(out, err)
	}

	cleanedContainerID := stripTrailingCharacters(out)

	inspectCmd := exec.Command(dockerBinary, "inspect", cleanedContainerID)
	out, _, err = runCommandWithOutput(inspectCmd)
	if err != nil {
		t.Fatalf("out should've been a container id: %s, %v", out, err)
	}

	containers := []struct {
		ID      string
		Created time.Time
		Path    string
		Args    []string
		Image   string
	}{}
	if err := json.Unmarshal([]byte(out), &containers); err != nil {
		t.Fatalf("Error inspecting the container: %s", err)
	}
	if len(containers) != 1 {
		t.Fatalf("Unexpected container count. Expected 0, received: %d", len(containers))
	}

	c := containers[0]
	if c.Path != "command" {
		t.Fatalf("Unexpected container path. Expected command, received: %s", c.Path)
	}

	b := false
	expected := []string{"arg1", "arg2", "arg with space"}
	for i, arg := range expected {
		if arg != c.Args[i] {
			b = true
			break
		}
	}
	if len(c.Args) != len(expected) || b {
		t.Fatalf("Unexpected args. Expected %v, received: %v", expected, c.Args)
	}

	deleteAllContainers()

	logDone("create - args")
}

// Make sure we can set hostconfig options too
func TestCreateHostConfig(t *testing.T) {
	runCmd := exec.Command(dockerBinary, "create", "-P", "busybox", "echo")
	out, _, _, err := runCommandWithStdoutStderr(runCmd)
	if err != nil {
		t.Fatal(out, err)
	}

	cleanedContainerID := stripTrailingCharacters(out)

	inspectCmd := exec.Command(dockerBinary, "inspect", cleanedContainerID)
	out, _, err = runCommandWithOutput(inspectCmd)
	if err != nil {
		t.Fatalf("out should've been a container id: %s, %v", out, err)
	}

	containers := []struct {
		HostConfig *struct {
			PublishAllPorts bool
		}
	}{}
	if err := json.Unmarshal([]byte(out), &containers); err != nil {
		t.Fatalf("Error inspecting the container: %s", err)
	}
	if len(containers) != 1 {
		t.Fatalf("Unexpected container count. Expected 0, received: %d", len(containers))
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

func TestCreateWithPortRange(t *testing.T) {
	runCmd := exec.Command(dockerBinary, "create", "-p", "3300-3303:3300-3303/tcp", "busybox", "echo")
	out, _, _, err := runCommandWithStdoutStderr(runCmd)
	if err != nil {
		t.Fatal(out, err)
	}

	cleanedContainerID := stripTrailingCharacters(out)

	inspectCmd := exec.Command(dockerBinary, "inspect", cleanedContainerID)
	out, _, err = runCommandWithOutput(inspectCmd)
	if err != nil {
		t.Fatalf("out should've been a container id: %s, %v", out, err)
	}

	containers := []struct {
		HostConfig *struct {
			PortBindings map[nat.Port][]nat.PortBinding
		}
	}{}
	if err := json.Unmarshal([]byte(out), &containers); err != nil {
		t.Fatalf("Error inspecting the container: %s", err)
	}
	if len(containers) != 1 {
		t.Fatalf("Unexpected container count. Expected 0, received: %d", len(containers))
	}

	c := containers[0]
	if c.HostConfig == nil {
		t.Fatalf("Expected HostConfig, got none")
	}

	if len(c.HostConfig.PortBindings) != 4 {
		t.Fatalf("Expected 4 ports bindings, got %d", len(c.HostConfig.PortBindings))
	}
	for k, v := range c.HostConfig.PortBindings {
		if len(v) != 1 {
			t.Fatalf("Expected 1 ports binding, for the port  %s but found %s", k, v)
		}
		if k.Port() != v[0].HostPort {
			t.Fatalf("Expected host port %d to match published port  %d", k.Port(), v[0].HostPort)
		}
	}

	deleteAllContainers()

	logDone("create - port range")
}

func TestCreateWithiLargePortRange(t *testing.T) {
	runCmd := exec.Command(dockerBinary, "create", "-p", "1-65535:1-65535/tcp", "busybox", "echo")
	out, _, _, err := runCommandWithStdoutStderr(runCmd)
	if err != nil {
		t.Fatal(out, err)
	}

	cleanedContainerID := stripTrailingCharacters(out)

	inspectCmd := exec.Command(dockerBinary, "inspect", cleanedContainerID)
	out, _, err = runCommandWithOutput(inspectCmd)
	if err != nil {
		t.Fatalf("out should've been a container id: %s, %v", out, err)
	}

	containers := []struct {
		HostConfig *struct {
			PortBindings map[nat.Port][]nat.PortBinding
		}
	}{}
	if err := json.Unmarshal([]byte(out), &containers); err != nil {
		t.Fatalf("Error inspecting the container: %s", err)
	}
	if len(containers) != 1 {
		t.Fatalf("Unexpected container count. Expected 0, received: %d", len(containers))
	}

	c := containers[0]
	if c.HostConfig == nil {
		t.Fatalf("Expected HostConfig, got none")
	}

	if len(c.HostConfig.PortBindings) != 65535 {
		t.Fatalf("Expected 65535 ports bindings, got %d", len(c.HostConfig.PortBindings))
	}
	for k, v := range c.HostConfig.PortBindings {
		if len(v) != 1 {
			t.Fatalf("Expected 1 ports binding, for the port  %s but found %s", k, v)
		}
		if k.Port() != v[0].HostPort {
			t.Fatalf("Expected host port %d to match published port  %d", k.Port(), v[0].HostPort)
		}
	}

	deleteAllContainers()

	logDone("create - large port range")
}

// "test123" should be printed by docker create + start
func TestCreateEchoStdout(t *testing.T) {
	runCmd := exec.Command(dockerBinary, "create", "busybox", "echo", "test123")
	out, _, _, err := runCommandWithStdoutStderr(runCmd)
	if err != nil {
		t.Fatal(out, err)
	}

	cleanedContainerID := stripTrailingCharacters(out)

	runCmd = exec.Command(dockerBinary, "start", "-ai", cleanedContainerID)
	out, _, _, err = runCommandWithStdoutStderr(runCmd)
	if err != nil {
		t.Fatal(out, err)
	}

	if out != "test123\n" {
		t.Errorf("container should've printed 'test123', got %q", out)
	}

	deleteAllContainers()

	logDone("create - echo test123")
}

func TestCreateVolumesCreated(t *testing.T) {
	name := "test_create_volume"
	if out, _, err := runCommandWithOutput(exec.Command(dockerBinary, "create", "--name", name, "-v", "/foo", "busybox")); err != nil {
		t.Fatal(out, err)
	}
	dir, err := inspectFieldMap(name, "Volumes", "/foo")
	if err != nil {
		t.Fatalf("Error getting volume host path: %q", err)
	}

	if _, err := os.Stat(dir); err != nil && os.IsNotExist(err) {
		t.Fatalf("Volume was not created")
	}
	if err != nil {
		t.Fatalf("Error statting volume host path: %q", err)
	}

	logDone("create - volumes are created")
}
