package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"reflect"
	"testing"
	"time"

	"github.com/docker/docker/nat"
)

// Make sure we can create a simple container with some args
func TestCreateArgs(t *testing.T) {
	defer deleteAllContainers()

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

	logDone("create - args")
}

// Make sure we can set hostconfig options too
func TestCreateHostConfig(t *testing.T) {
	defer deleteAllContainers()

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

	logDone("create - hostconfig")
}

func TestCreateWithPortRange(t *testing.T) {
	defer deleteAllContainers()

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

	logDone("create - port range")
}

func TestCreateWithiLargePortRange(t *testing.T) {
	defer deleteAllContainers()

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

	logDone("create - large port range")
}

// "test123" should be printed by docker create + start
func TestCreateEchoStdout(t *testing.T) {
	defer deleteAllContainers()

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

	logDone("create - echo test123")
}

func TestCreateVolumesCreated(t *testing.T) {
	testRequires(t, SameHostDaemon)
	defer deleteAllContainers()

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

func TestCreateLabels(t *testing.T) {
	name := "test_create_labels"
	expected := map[string]string{"k1": "v1", "k2": "v2"}
	if out, _, err := runCommandWithOutput(exec.Command(dockerBinary, "create", "--name", name, "-l", "k1=v1", "--label", "k2=v2", "busybox")); err != nil {
		t.Fatal(out, err)
	}

	actual := make(map[string]string)
	err := inspectFieldAndMarshall(name, "Config.Labels", &actual)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(expected, actual) {
		t.Fatalf("Expected %s got %s", expected, actual)
	}

	deleteAllContainers()

	logDone("create - labels")
}

func TestCreateLabelFromImage(t *testing.T) {
	imageName := "testcreatebuildlabel"
	defer deleteImages(imageName)
	_, err := buildImage(imageName,
		`FROM busybox
		LABEL k1=v1 k2=v2`,
		true)
	if err != nil {
		t.Fatal(err)
	}

	name := "test_create_labels_from_image"
	expected := map[string]string{"k2": "x", "k3": "v3", "k1": "v1"}
	if out, _, err := runCommandWithOutput(exec.Command(dockerBinary, "create", "--name", name, "-l", "k2=x", "--label", "k3=v3", imageName)); err != nil {
		t.Fatal(out, err)
	}

	actual := make(map[string]string)
	err = inspectFieldAndMarshall(name, "Config.Labels", &actual)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(expected, actual) {
		t.Fatalf("Expected %s got %s", expected, actual)
	}

	deleteAllContainers()

	logDone("create - labels from image")
}
