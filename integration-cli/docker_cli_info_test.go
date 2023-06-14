package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"gotest.tools/v3/assert"
)

type DockerCLIInfoSuite struct {
	ds *DockerSuite
}

func (s *DockerCLIInfoSuite) TearDownTest(c *testing.T) {
	s.ds.TearDownTest(c)
}

func (s *DockerCLIInfoSuite) OnTimeout(c *testing.T) {
	s.ds.OnTimeout(c)
}

// ensure docker info succeeds
func (s *DockerCLIInfoSuite) TestInfoEnsureSucceeds(c *testing.T) {
	out, _ := dockerCmd(c, "info")

	// always shown fields
	stringsToCheck := []string{
		"ID:",
		"Containers:",
		" Running:",
		" Paused:",
		" Stopped:",
		"Images:",
		"OSType:",
		"Architecture:",
		"Logging Driver:",
		"Operating System:",
		"CPUs:",
		"Total Memory:",
		"Kernel Version:",
		"Storage Driver:",
		"Volume:",
		"Network:",
		"Live Restore Enabled:",
	}

	if testEnv.DaemonInfo.OSType == "linux" {
		stringsToCheck = append(stringsToCheck, "Init Binary:", "Security Options:", "containerd version:", "runc version:", "init version:")
	}

	if DaemonIsLinux() {
		stringsToCheck = append(stringsToCheck, "Runtimes:", "Default Runtime: runc")
	}

	if testEnv.DaemonInfo.ExperimentalBuild {
		stringsToCheck = append(stringsToCheck, "Experimental: true")
	} else {
		stringsToCheck = append(stringsToCheck, "Experimental: false")
	}

	for _, linePrefix := range stringsToCheck {
		assert.Assert(c, strings.Contains(out, linePrefix), "couldn't find string %v in output", linePrefix)
	}
}

func (s *DockerCLIInfoSuite) TestInfoDisplaysRunningContainers(c *testing.T) {
	testRequires(c, DaemonIsLinux)

	existing := existingContainerStates(c)

	dockerCmd(c, "run", "-d", "busybox", "top")
	out, _ := dockerCmd(c, "info")
	assert.Assert(c, strings.Contains(out, fmt.Sprintf("Containers: %d\n", existing["Containers"]+1)))
	assert.Assert(c, strings.Contains(out, fmt.Sprintf(" Running: %d\n", existing["ContainersRunning"]+1)))
	assert.Assert(c, strings.Contains(out, fmt.Sprintf(" Paused: %d\n", existing["ContainersPaused"])))
	assert.Assert(c, strings.Contains(out, fmt.Sprintf(" Stopped: %d\n", existing["ContainersStopped"])))
}

func (s *DockerCLIInfoSuite) TestInfoDisplaysPausedContainers(c *testing.T) {
	testRequires(c, IsPausable)

	existing := existingContainerStates(c)

	out := runSleepingContainer(c, "-d")
	cleanedContainerID := strings.TrimSpace(out)

	dockerCmd(c, "pause", cleanedContainerID)

	out, _ = dockerCmd(c, "info")
	assert.Assert(c, strings.Contains(out, fmt.Sprintf("Containers: %d\n", existing["Containers"]+1)))
	assert.Assert(c, strings.Contains(out, fmt.Sprintf(" Running: %d\n", existing["ContainersRunning"])))
	assert.Assert(c, strings.Contains(out, fmt.Sprintf(" Paused: %d\n", existing["ContainersPaused"]+1)))
	assert.Assert(c, strings.Contains(out, fmt.Sprintf(" Stopped: %d\n", existing["ContainersStopped"])))
}

func (s *DockerCLIInfoSuite) TestInfoDisplaysStoppedContainers(c *testing.T) {
	testRequires(c, DaemonIsLinux)

	existing := existingContainerStates(c)

	out, _ := dockerCmd(c, "run", "-d", "busybox", "top")
	cleanedContainerID := strings.TrimSpace(out)

	dockerCmd(c, "stop", cleanedContainerID)

	out, _ = dockerCmd(c, "info")
	assert.Assert(c, strings.Contains(out, fmt.Sprintf("Containers: %d\n", existing["Containers"]+1)))
	assert.Assert(c, strings.Contains(out, fmt.Sprintf(" Running: %d\n", existing["ContainersRunning"])))
	assert.Assert(c, strings.Contains(out, fmt.Sprintf(" Paused: %d\n", existing["ContainersPaused"])))
	assert.Assert(c, strings.Contains(out, fmt.Sprintf(" Stopped: %d\n", existing["ContainersStopped"]+1)))
}

func existingContainerStates(c *testing.T) map[string]int {
	out, _ := dockerCmd(c, "info", "--format", "{{json .}}")
	var m map[string]interface{}
	err := json.Unmarshal([]byte(out), &m)
	assert.NilError(c, err)
	res := map[string]int{}
	res["Containers"] = int(m["Containers"].(float64))
	res["ContainersRunning"] = int(m["ContainersRunning"].(float64))
	res["ContainersPaused"] = int(m["ContainersPaused"].(float64))
	res["ContainersStopped"] = int(m["ContainersStopped"].(float64))
	return res
}
