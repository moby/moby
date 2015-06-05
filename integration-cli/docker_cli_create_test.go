package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"time"

	"github.com/docker/docker/nat"
	"github.com/go-check/check"
)

// Make sure we can create a simple container with some args
func (s *DockerSuite) TestCreateArgs(c *check.C) {
	runCmd := exec.Command(dockerBinary, "create", "busybox", "command", "arg1", "arg2", "arg with space")
	out, _, _, err := runCommandWithStdoutStderr(runCmd)
	if err != nil {
		c.Fatal(out, err)
	}

	cleanedContainerID := strings.TrimSpace(out)

	inspectCmd := exec.Command(dockerBinary, "inspect", cleanedContainerID)
	out, _, err = runCommandWithOutput(inspectCmd)
	if err != nil {
		c.Fatalf("out should've been a container id: %s, %v", out, err)
	}

	containers := []struct {
		ID      string
		Created time.Time
		Path    string
		Args    []string
		Image   string
	}{}
	if err := json.Unmarshal([]byte(out), &containers); err != nil {
		c.Fatalf("Error inspecting the container: %s", err)
	}
	if len(containers) != 1 {
		c.Fatalf("Unexpected container count. Expected 0, received: %d", len(containers))
	}

	cont := containers[0]
	if cont.Path != "command" {
		c.Fatalf("Unexpected container path. Expected command, received: %s", cont.Path)
	}

	b := false
	expected := []string{"arg1", "arg2", "arg with space"}
	for i, arg := range expected {
		if arg != cont.Args[i] {
			b = true
			break
		}
	}
	if len(cont.Args) != len(expected) || b {
		c.Fatalf("Unexpected args. Expected %v, received: %v", expected, cont.Args)
	}

}

// Make sure we can set hostconfig options too
func (s *DockerSuite) TestCreateHostConfig(c *check.C) {

	runCmd := exec.Command(dockerBinary, "create", "-P", "busybox", "echo")
	out, _, _, err := runCommandWithStdoutStderr(runCmd)
	if err != nil {
		c.Fatal(out, err)
	}

	cleanedContainerID := strings.TrimSpace(out)

	inspectCmd := exec.Command(dockerBinary, "inspect", cleanedContainerID)
	out, _, err = runCommandWithOutput(inspectCmd)
	if err != nil {
		c.Fatalf("out should've been a container id: %s, %v", out, err)
	}

	containers := []struct {
		HostConfig *struct {
			PublishAllPorts bool
		}
	}{}
	if err := json.Unmarshal([]byte(out), &containers); err != nil {
		c.Fatalf("Error inspecting the container: %s", err)
	}
	if len(containers) != 1 {
		c.Fatalf("Unexpected container count. Expected 0, received: %d", len(containers))
	}

	cont := containers[0]
	if cont.HostConfig == nil {
		c.Fatalf("Expected HostConfig, got none")
	}

	if !cont.HostConfig.PublishAllPorts {
		c.Fatalf("Expected PublishAllPorts, got false")
	}

}

func (s *DockerSuite) TestCreateWithPortRange(c *check.C) {

	runCmd := exec.Command(dockerBinary, "create", "-p", "3300-3303:3300-3303/tcp", "busybox", "echo")
	out, _, _, err := runCommandWithStdoutStderr(runCmd)
	if err != nil {
		c.Fatal(out, err)
	}

	cleanedContainerID := strings.TrimSpace(out)

	inspectCmd := exec.Command(dockerBinary, "inspect", cleanedContainerID)
	out, _, err = runCommandWithOutput(inspectCmd)
	if err != nil {
		c.Fatalf("out should've been a container id: %s, %v", out, err)
	}

	containers := []struct {
		HostConfig *struct {
			PortBindings map[nat.Port][]nat.PortBinding
		}
	}{}
	if err := json.Unmarshal([]byte(out), &containers); err != nil {
		c.Fatalf("Error inspecting the container: %s", err)
	}
	if len(containers) != 1 {
		c.Fatalf("Unexpected container count. Expected 0, received: %d", len(containers))
	}

	cont := containers[0]
	if cont.HostConfig == nil {
		c.Fatalf("Expected HostConfig, got none")
	}

	if len(cont.HostConfig.PortBindings) != 4 {
		c.Fatalf("Expected 4 ports bindings, got %d", len(cont.HostConfig.PortBindings))
	}
	for k, v := range cont.HostConfig.PortBindings {
		if len(v) != 1 {
			c.Fatalf("Expected 1 ports binding, for the port  %s but found %s", k, v)
		}
		if k.Port() != v[0].HostPort {
			c.Fatalf("Expected host port %d to match published port  %d", k.Port(), v[0].HostPort)
		}
	}

}

func (s *DockerSuite) TestCreateWithiLargePortRange(c *check.C) {

	runCmd := exec.Command(dockerBinary, "create", "-p", "1-65535:1-65535/tcp", "busybox", "echo")
	out, _, _, err := runCommandWithStdoutStderr(runCmd)
	if err != nil {
		c.Fatal(out, err)
	}

	cleanedContainerID := strings.TrimSpace(out)

	inspectCmd := exec.Command(dockerBinary, "inspect", cleanedContainerID)
	out, _, err = runCommandWithOutput(inspectCmd)
	if err != nil {
		c.Fatalf("out should've been a container id: %s, %v", out, err)
	}

	containers := []struct {
		HostConfig *struct {
			PortBindings map[nat.Port][]nat.PortBinding
		}
	}{}
	if err := json.Unmarshal([]byte(out), &containers); err != nil {
		c.Fatalf("Error inspecting the container: %s", err)
	}
	if len(containers) != 1 {
		c.Fatalf("Unexpected container count. Expected 0, received: %d", len(containers))
	}

	cont := containers[0]
	if cont.HostConfig == nil {
		c.Fatalf("Expected HostConfig, got none")
	}

	if len(cont.HostConfig.PortBindings) != 65535 {
		c.Fatalf("Expected 65535 ports bindings, got %d", len(cont.HostConfig.PortBindings))
	}
	for k, v := range cont.HostConfig.PortBindings {
		if len(v) != 1 {
			c.Fatalf("Expected 1 ports binding, for the port  %s but found %s", k, v)
		}
		if k.Port() != v[0].HostPort {
			c.Fatalf("Expected host port %d to match published port  %d", k.Port(), v[0].HostPort)
		}
	}

}

// "test123" should be printed by docker create + start
func (s *DockerSuite) TestCreateEchoStdout(c *check.C) {

	runCmd := exec.Command(dockerBinary, "create", "busybox", "echo", "test123")
	out, _, _, err := runCommandWithStdoutStderr(runCmd)
	if err != nil {
		c.Fatal(out, err)
	}

	cleanedContainerID := strings.TrimSpace(out)

	runCmd = exec.Command(dockerBinary, "start", "-ai", cleanedContainerID)
	out, _, _, err = runCommandWithStdoutStderr(runCmd)
	if err != nil {
		c.Fatal(out, err)
	}

	if out != "test123\n" {
		c.Errorf("container should've printed 'test123', got %q", out)
	}

}

func (s *DockerSuite) TestCreateVolumesCreated(c *check.C) {
	testRequires(c, SameHostDaemon)

	name := "test_create_volume"
	if out, _, err := runCommandWithOutput(exec.Command(dockerBinary, "create", "--name", name, "-v", "/foo", "busybox")); err != nil {
		c.Fatal(out, err)
	}
	dir, err := inspectFieldMap(name, "Volumes", "/foo")
	if err != nil {
		c.Fatalf("Error getting volume host path: %q", err)
	}

	if _, err := os.Stat(dir); err != nil && os.IsNotExist(err) {
		c.Fatalf("Volume was not created")
	}
	if err != nil {
		c.Fatalf("Error statting volume host path: %q", err)
	}

}

func (s *DockerSuite) TestCreateLabels(c *check.C) {
	name := "test_create_labels"
	expected := map[string]string{"k1": "v1", "k2": "v2"}
	if out, _, err := runCommandWithOutput(exec.Command(dockerBinary, "create", "--name", name, "-l", "k1=v1", "--label", "k2=v2", "busybox")); err != nil {
		c.Fatal(out, err)
	}

	actual := make(map[string]string)
	err := inspectFieldAndMarshall(name, "Config.Labels", &actual)
	if err != nil {
		c.Fatal(err)
	}

	if !reflect.DeepEqual(expected, actual) {
		c.Fatalf("Expected %s got %s", expected, actual)
	}
}

func (s *DockerSuite) TestCreateLabelFromImage(c *check.C) {
	imageName := "testcreatebuildlabel"
	_, err := buildImage(imageName,
		`FROM busybox
		LABEL k1=v1 k2=v2`,
		true)
	if err != nil {
		c.Fatal(err)
	}

	name := "test_create_labels_from_image"
	expected := map[string]string{"k2": "x", "k3": "v3"}
	if out, _, err := runCommandWithOutput(exec.Command(dockerBinary, "create", "--name", name, "-l", "k2=x", "--label", "k3=v3", imageName)); err != nil {
		c.Fatal(out, err)
	}

	actual := make(map[string]string)
	err = inspectFieldAndMarshall(name, "Config.Labels", &actual)
	if err != nil {
		c.Fatal(err)
	}

	if !reflect.DeepEqual(expected, actual) {
		c.Fatalf("Expected %s got %s", expected, actual)
	}
}

func (s *DockerSuite) TestCreateHostnameWithNumber(c *check.C) {
	out, _ := dockerCmd(c, "run", "-h", "web.0", "busybox", "hostname")
	if strings.TrimSpace(out) != "web.0" {
		c.Fatalf("hostname not set, expected `web.0`, got: %s", out)
	}
}

func (s *DockerSuite) TestCreateRM(c *check.C) {
	// Test to make sure we can 'rm' a new container that is in
	// "Created" state, and has ever been run. Test "rm -f" too.

	// create a container
	createCmd := exec.Command(dockerBinary, "create", "busybox")
	out, _, err := runCommandWithOutput(createCmd)
	if err != nil {
		c.Fatalf("Failed to create container:%s\n%s", out, err)
	}
	cID := strings.TrimSpace(out)

	rmCmd := exec.Command(dockerBinary, "rm", cID)
	out, _, err = runCommandWithOutput(rmCmd)
	if err != nil {
		c.Fatalf("Failed to rm container:%s\n%s", out, err)
	}

	// Now do it again so we can "rm -f" this time
	createCmd = exec.Command(dockerBinary, "create", "busybox")
	out, _, err = runCommandWithOutput(createCmd)
	if err != nil {
		c.Fatalf("Failed to create 2nd container:%s\n%s", out, err)
	}

	cID = strings.TrimSpace(out)
	rmCmd = exec.Command(dockerBinary, "rm", "-f", cID)
	out, _, err = runCommandWithOutput(rmCmd)
	if err != nil {
		c.Fatalf("Failed to rm -f container:%s\n%s", out, err)
	}
}

func (s *DockerSuite) TestCreateModeIpcContainer(c *check.C) {
	testRequires(c, SameHostDaemon)

	cmd := exec.Command(dockerBinary, "create", "busybox")
	out, _, err := runCommandWithOutput(cmd)
	if err != nil {
		c.Fatal(err, out)
	}
	id := strings.TrimSpace(out)

	cmd = exec.Command(dockerBinary, "create", fmt.Sprintf("--ipc=container:%s", id), "busybox")
	out, _, err = runCommandWithOutput(cmd)
	if err != nil {
		c.Fatalf("Create container with ipc mode container should success with non running container: %s\n%s", out, err)
	}
}
