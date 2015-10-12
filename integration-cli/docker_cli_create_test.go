package main

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"time"

	"os/exec"

	"io/ioutil"

	"github.com/docker/docker/pkg/nat"
	"github.com/go-check/check"
)

// Make sure we can create a simple container with some args
func (s *DockerSuite) TestCreateArgs(c *check.C) {
	testRequires(c, DaemonIsLinux)
	out, _ := dockerCmd(c, "create", "busybox", "command", "arg1", "arg2", "arg with space")

	cleanedContainerID := strings.TrimSpace(out)

	out, _ = dockerCmd(c, "inspect", cleanedContainerID)

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
	testRequires(c, DaemonIsLinux)
	out, _ := dockerCmd(c, "create", "-P", "busybox", "echo")

	cleanedContainerID := strings.TrimSpace(out)

	out, _ = dockerCmd(c, "inspect", cleanedContainerID)

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
	testRequires(c, DaemonIsLinux)
	out, _ := dockerCmd(c, "create", "-p", "3300-3303:3300-3303/tcp", "busybox", "echo")

	cleanedContainerID := strings.TrimSpace(out)

	out, _ = dockerCmd(c, "inspect", cleanedContainerID)

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
			c.Fatalf("Expected host port %s to match published port %s", k.Port(), v[0].HostPort)
		}
	}

}

func (s *DockerSuite) TestCreateWithiLargePortRange(c *check.C) {
	testRequires(c, DaemonIsLinux)
	out, _ := dockerCmd(c, "create", "-p", "1-65535:1-65535/tcp", "busybox", "echo")

	cleanedContainerID := strings.TrimSpace(out)

	out, _ = dockerCmd(c, "inspect", cleanedContainerID)

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
			c.Fatalf("Expected host port %s to match published port %s", k.Port(), v[0].HostPort)
		}
	}

}

// "test123" should be printed by docker create + start
func (s *DockerSuite) TestCreateEchoStdout(c *check.C) {
	testRequires(c, DaemonIsLinux)

	out, _ := dockerCmd(c, "create", "busybox", "echo", "test123")

	cleanedContainerID := strings.TrimSpace(out)

	out, _ = dockerCmd(c, "start", "-ai", cleanedContainerID)

	if out != "test123\n" {
		c.Errorf("container should've printed 'test123', got %q", out)
	}

}

func (s *DockerSuite) TestCreateVolumesCreated(c *check.C) {
	testRequires(c, DaemonIsLinux)
	testRequires(c, SameHostDaemon)

	name := "test_create_volume"
	dockerCmd(c, "create", "--name", name, "-v", "/foo", "busybox")

	dir, err := inspectMountSourceField(name, "/foo")
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
	testRequires(c, DaemonIsLinux)
	name := "test_create_labels"
	expected := map[string]string{"k1": "v1", "k2": "v2"}
	dockerCmd(c, "create", "--name", name, "-l", "k1=v1", "--label", "k2=v2", "busybox")

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
	testRequires(c, DaemonIsLinux)
	imageName := "testcreatebuildlabel"
	_, err := buildImage(imageName,
		`FROM busybox
		LABEL k1=v1 k2=v2`,
		true)
	if err != nil {
		c.Fatal(err)
	}

	name := "test_create_labels_from_image"
	expected := map[string]string{"k2": "x", "k3": "v3", "k1": "v1"}
	dockerCmd(c, "create", "--name", name, "-l", "k2=x", "--label", "k3=v3", imageName)

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
	testRequires(c, DaemonIsLinux)
	out, _ := dockerCmd(c, "run", "-h", "web.0", "busybox", "hostname")
	if strings.TrimSpace(out) != "web.0" {
		c.Fatalf("hostname not set, expected `web.0`, got: %s", out)
	}
}

func (s *DockerSuite) TestCreateRM(c *check.C) {
	testRequires(c, DaemonIsLinux)
	// Test to make sure we can 'rm' a new container that is in
	// "Created" state, and has ever been run. Test "rm -f" too.

	// create a container
	out, _ := dockerCmd(c, "create", "busybox")
	cID := strings.TrimSpace(out)

	dockerCmd(c, "rm", cID)

	// Now do it again so we can "rm -f" this time
	out, _ = dockerCmd(c, "create", "busybox")

	cID = strings.TrimSpace(out)
	dockerCmd(c, "rm", "-f", cID)
}

func (s *DockerSuite) TestCreateModeIpcContainer(c *check.C) {
	testRequires(c, DaemonIsLinux)
	testRequires(c, SameHostDaemon, NotUserNamespace)

	out, _ := dockerCmd(c, "create", "busybox")
	id := strings.TrimSpace(out)

	dockerCmd(c, "create", fmt.Sprintf("--ipc=container:%s", id), "busybox")
}

func (s *DockerTrustSuite) TestTrustedCreate(c *check.C) {
	repoName := s.setupTrustedImage(c, "trusted-create")

	// Try create
	createCmd := exec.Command(dockerBinary, "create", repoName)
	s.trustedCmd(createCmd)
	out, _, err := runCommandWithOutput(createCmd)
	if err != nil {
		c.Fatalf("Error running trusted create: %s\n%s", err, out)
	}

	if !strings.Contains(string(out), "Tagging") {
		c.Fatalf("Missing expected output on trusted push:\n%s", out)
	}

	dockerCmd(c, "rmi", repoName)

	// Try untrusted create to ensure we pushed the tag to the registry
	createCmd = exec.Command(dockerBinary, "create", "--disable-content-trust=true", repoName)
	s.trustedCmd(createCmd)
	out, _, err = runCommandWithOutput(createCmd)
	if err != nil {
		c.Fatalf("Error running trusted create: %s\n%s", err, out)
	}

	if !strings.Contains(string(out), "Status: Downloaded") {
		c.Fatalf("Missing expected output on trusted create with --disable-content-trust:\n%s", out)
	}
}

func (s *DockerTrustSuite) TestUntrustedCreate(c *check.C) {
	repoName := fmt.Sprintf("%v/dockercli/trusted:latest", privateRegistryURL)
	// tag the image and upload it to the private registry
	dockerCmd(c, "tag", "busybox", repoName)
	dockerCmd(c, "push", repoName)
	dockerCmd(c, "rmi", repoName)

	// Try trusted create on untrusted tag
	createCmd := exec.Command(dockerBinary, "create", repoName)
	s.trustedCmd(createCmd)
	out, _, err := runCommandWithOutput(createCmd)
	if err == nil {
		c.Fatalf("Error expected when running trusted create with:\n%s", out)
	}

	if !strings.Contains(string(out), "no trust data available") {
		c.Fatalf("Missing expected output on trusted create:\n%s", out)
	}
}

func (s *DockerTrustSuite) TestTrustedIsolatedCreate(c *check.C) {
	repoName := s.setupTrustedImage(c, "trusted-isolated-create")

	// Try create
	createCmd := exec.Command(dockerBinary, "--config", "/tmp/docker-isolated-create", "create", repoName)
	s.trustedCmd(createCmd)
	out, _, err := runCommandWithOutput(createCmd)
	if err != nil {
		c.Fatalf("Error running trusted create: %s\n%s", err, out)
	}

	if !strings.Contains(string(out), "Tagging") {
		c.Fatalf("Missing expected output on trusted push:\n%s", out)
	}

	dockerCmd(c, "rmi", repoName)
}

func (s *DockerTrustSuite) TestCreateWhenCertExpired(c *check.C) {
	c.Skip("Currently changes system time, causing instability")
	repoName := s.setupTrustedImage(c, "trusted-create-expired")

	// Certificates have 10 years of expiration
	elevenYearsFromNow := time.Now().Add(time.Hour * 24 * 365 * 11)

	runAtDifferentDate(elevenYearsFromNow, func() {
		// Try create
		createCmd := exec.Command(dockerBinary, "create", repoName)
		s.trustedCmd(createCmd)
		out, _, err := runCommandWithOutput(createCmd)
		if err == nil {
			c.Fatalf("Error running trusted create in the distant future: %s\n%s", err, out)
		}

		if !strings.Contains(string(out), "could not validate the path to a trusted root") {
			c.Fatalf("Missing expected output on trusted create in the distant future:\n%s", out)
		}
	})

	runAtDifferentDate(elevenYearsFromNow, func() {
		// Try create
		createCmd := exec.Command(dockerBinary, "create", "--disable-content-trust", repoName)
		s.trustedCmd(createCmd)
		out, _, err := runCommandWithOutput(createCmd)
		if err != nil {
			c.Fatalf("Error running untrusted create in the distant future: %s\n%s", err, out)
		}

		if !strings.Contains(string(out), "Status: Downloaded") {
			c.Fatalf("Missing expected output on untrusted create in the distant future:\n%s", out)
		}
	})
}

func (s *DockerTrustSuite) TestTrustedCreateFromBadTrustServer(c *check.C) {
	repoName := fmt.Sprintf("%v/dockerclievilcreate/trusted:latest", privateRegistryURL)
	evilLocalConfigDir, err := ioutil.TempDir("", "evil-local-config-dir")
	if err != nil {
		c.Fatalf("Failed to create local temp dir")
	}

	// tag the image and upload it to the private registry
	dockerCmd(c, "tag", "busybox", repoName)

	pushCmd := exec.Command(dockerBinary, "push", repoName)
	s.trustedCmd(pushCmd)
	out, _, err := runCommandWithOutput(pushCmd)
	if err != nil {
		c.Fatalf("Error creating trusted push: %s\n%s", err, out)
	}
	if !strings.Contains(string(out), "Signing and pushing trust metadata") {
		c.Fatalf("Missing expected output on trusted push:\n%s", out)
	}

	dockerCmd(c, "rmi", repoName)

	// Try create
	createCmd := exec.Command(dockerBinary, "create", repoName)
	s.trustedCmd(createCmd)
	out, _, err = runCommandWithOutput(createCmd)
	if err != nil {
		c.Fatalf("Error creating trusted create: %s\n%s", err, out)
	}

	if !strings.Contains(string(out), "Tagging") {
		c.Fatalf("Missing expected output on trusted push:\n%s", out)
	}

	dockerCmd(c, "rmi", repoName)

	// Kill the notary server, start a new "evil" one.
	s.not.Close()
	s.not, err = newTestNotary(c)
	if err != nil {
		c.Fatalf("Restarting notary server failed.")
	}

	// In order to make an evil server, lets re-init a client (with a different trust dir) and push new data.
	// tag an image and upload it to the private registry
	dockerCmd(c, "--config", evilLocalConfigDir, "tag", "busybox", repoName)

	// Push up to the new server
	pushCmd = exec.Command(dockerBinary, "--config", evilLocalConfigDir, "push", repoName)
	s.trustedCmd(pushCmd)
	out, _, err = runCommandWithOutput(pushCmd)
	if err != nil {
		c.Fatalf("Error creating trusted push: %s\n%s", err, out)
	}
	if !strings.Contains(string(out), "Signing and pushing trust metadata") {
		c.Fatalf("Missing expected output on trusted push:\n%s", out)
	}

	// Now, try creating with the original client from this new trust server. This should fail.
	createCmd = exec.Command(dockerBinary, "create", repoName)
	s.trustedCmd(createCmd)
	out, _, err = runCommandWithOutput(createCmd)
	if err == nil {
		c.Fatalf("Expected to fail on this create due to different remote data: %s\n%s", err, out)
	}

	if !strings.Contains(string(out), "failed to validate data with current trusted certificates") {
		c.Fatalf("Missing expected output on trusted push:\n%s", out)
	}
}

func (s *DockerSuite) TestCreateStopSignal(c *check.C) {
	name := "test_create_stop_signal"
	dockerCmd(c, "create", "--name", name, "--stop-signal", "9", "busybox")

	res, err := inspectFieldJSON(name, "Config.StopSignal")
	c.Assert(err, check.IsNil)

	if res != `"9"` {
		c.Fatalf("Expected 9, got %s", res)
	}
}
