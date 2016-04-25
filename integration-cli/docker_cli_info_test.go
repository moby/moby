package main

import (
	"fmt"
	"net"
	"strings"

	"github.com/docker/docker/pkg/integration/checker"
	"github.com/docker/docker/utils"
	"github.com/go-check/check"
)

// ensure docker info succeeds
func (s *DockerSuite) TestInfoEnsureSucceeds(c *check.C) {
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
	}

	if utils.ExperimentalBuild() {
		stringsToCheck = append(stringsToCheck, "Experimental: true")
	}

	for _, linePrefix := range stringsToCheck {
		c.Assert(out, checker.Contains, linePrefix, check.Commentf("couldn't find string %v in output", linePrefix))
	}
}

// TestInfoDiscoveryBackend verifies that a daemon run with `--cluster-advertise` and
// `--cluster-store` properly show the backend's endpoint in info output.
func (s *DockerSuite) TestInfoDiscoveryBackend(c *check.C) {
	testRequires(c, SameHostDaemon, DaemonIsLinux)

	d := NewDaemon(c)
	discoveryBackend := "consul://consuladdr:consulport/some/path"
	discoveryAdvertise := "1.1.1.1:2375"
	err := d.Start(fmt.Sprintf("--cluster-store=%s", discoveryBackend), fmt.Sprintf("--cluster-advertise=%s", discoveryAdvertise))
	c.Assert(err, checker.IsNil)
	defer d.Stop()

	out, err := d.Cmd("info")
	c.Assert(err, checker.IsNil)
	c.Assert(out, checker.Contains, fmt.Sprintf("Cluster Store: %s\n", discoveryBackend))
	c.Assert(out, checker.Contains, fmt.Sprintf("Cluster Advertise: %s\n", discoveryAdvertise))
}

// TestInfoDiscoveryInvalidAdvertise verifies that a daemon run with
// an invalid `--cluster-advertise` configuration
func (s *DockerSuite) TestInfoDiscoveryInvalidAdvertise(c *check.C) {
	testRequires(c, SameHostDaemon, DaemonIsLinux)

	d := NewDaemon(c)
	discoveryBackend := "consul://consuladdr:consulport/some/path"

	// --cluster-advertise with an invalid string is an error
	err := d.Start(fmt.Sprintf("--cluster-store=%s", discoveryBackend), "--cluster-advertise=invalid")
	c.Assert(err, checker.Not(checker.IsNil))

	// --cluster-advertise without --cluster-store is also an error
	err = d.Start("--cluster-advertise=1.1.1.1:2375")
	c.Assert(err, checker.Not(checker.IsNil))
}

// TestInfoDiscoveryAdvertiseInterfaceName verifies that a daemon run with `--cluster-advertise`
// configured with interface name properly show the advertise ip-address in info output.
func (s *DockerSuite) TestInfoDiscoveryAdvertiseInterfaceName(c *check.C) {
	testRequires(c, SameHostDaemon, Network, DaemonIsLinux)

	d := NewDaemon(c)
	discoveryBackend := "consul://consuladdr:consulport/some/path"
	discoveryAdvertise := "eth0"

	err := d.Start(fmt.Sprintf("--cluster-store=%s", discoveryBackend), fmt.Sprintf("--cluster-advertise=%s:2375", discoveryAdvertise))
	c.Assert(err, checker.IsNil)
	defer d.Stop()

	iface, err := net.InterfaceByName(discoveryAdvertise)
	c.Assert(err, checker.IsNil)
	addrs, err := iface.Addrs()
	c.Assert(err, checker.IsNil)
	c.Assert(len(addrs), checker.GreaterThan, 0)
	ip, _, err := net.ParseCIDR(addrs[0].String())
	c.Assert(err, checker.IsNil)

	out, err := d.Cmd("info")
	c.Assert(err, checker.IsNil)
	c.Assert(out, checker.Contains, fmt.Sprintf("Cluster Store: %s\n", discoveryBackend))
	c.Assert(out, checker.Contains, fmt.Sprintf("Cluster Advertise: %s:2375\n", ip.String()))
}

func (s *DockerSuite) TestInfoDisplaysRunningContainers(c *check.C) {
	testRequires(c, DaemonIsLinux)

	dockerCmd(c, "run", "-d", "busybox", "top")
	out, _ := dockerCmd(c, "info")
	c.Assert(out, checker.Contains, fmt.Sprintf("Containers: %d\n", 1))
	c.Assert(out, checker.Contains, fmt.Sprintf(" Running: %d\n", 1))
	c.Assert(out, checker.Contains, fmt.Sprintf(" Paused: %d\n", 0))
	c.Assert(out, checker.Contains, fmt.Sprintf(" Stopped: %d\n", 0))
}

func (s *DockerSuite) TestInfoDisplaysPausedContainers(c *check.C) {
	testRequires(c, DaemonIsLinux)

	out, _ := dockerCmd(c, "run", "-d", "busybox", "top")
	cleanedContainerID := strings.TrimSpace(out)

	dockerCmd(c, "pause", cleanedContainerID)

	out, _ = dockerCmd(c, "info")
	c.Assert(out, checker.Contains, fmt.Sprintf("Containers: %d\n", 1))
	c.Assert(out, checker.Contains, fmt.Sprintf(" Running: %d\n", 0))
	c.Assert(out, checker.Contains, fmt.Sprintf(" Paused: %d\n", 1))
	c.Assert(out, checker.Contains, fmt.Sprintf(" Stopped: %d\n", 0))
}

func (s *DockerSuite) TestInfoDisplaysStoppedContainers(c *check.C) {
	testRequires(c, DaemonIsLinux)

	out, _ := dockerCmd(c, "run", "-d", "busybox", "top")
	cleanedContainerID := strings.TrimSpace(out)

	dockerCmd(c, "stop", cleanedContainerID)

	out, _ = dockerCmd(c, "info")
	c.Assert(out, checker.Contains, fmt.Sprintf("Containers: %d\n", 1))
	c.Assert(out, checker.Contains, fmt.Sprintf(" Running: %d\n", 0))
	c.Assert(out, checker.Contains, fmt.Sprintf(" Paused: %d\n", 0))
	c.Assert(out, checker.Contains, fmt.Sprintf(" Stopped: %d\n", 1))
}

func (s *DockerSuite) TestInfoDebug(c *check.C) {
	testRequires(c, SameHostDaemon, DaemonIsLinux)

	d := NewDaemon(c)
	err := d.Start("--debug")
	c.Assert(err, checker.IsNil)
	defer d.Stop()

	out, err := d.Cmd("--debug", "info")
	c.Assert(err, checker.IsNil)
	c.Assert(out, checker.Contains, "Debug Mode (client): true\n")
	c.Assert(out, checker.Contains, "Debug Mode (server): true\n")
	c.Assert(out, checker.Contains, "File Descriptors")
	c.Assert(out, checker.Contains, "Goroutines")
	c.Assert(out, checker.Contains, "System Time")
	c.Assert(out, checker.Contains, "EventsListeners")
	c.Assert(out, checker.Contains, "Docker Root Dir")
}

func (s *DockerSuite) TestInsecureRegistries(c *check.C) {
	testRequires(c, SameHostDaemon, DaemonIsLinux)

	registryCIDR := "192.168.1.0/24"
	registryHost := "insecurehost.com:5000"

	d := NewDaemon(c)
	err := d.Start("--insecure-registry="+registryCIDR, "--insecure-registry="+registryHost)
	c.Assert(err, checker.IsNil)
	defer d.Stop()

	out, err := d.Cmd("info")
	c.Assert(err, checker.IsNil)
	c.Assert(out, checker.Contains, "Insecure Registries:\n")
	c.Assert(out, checker.Contains, fmt.Sprintf(" %s\n", registryHost))
	c.Assert(out, checker.Contains, fmt.Sprintf(" %s\n", registryCIDR))
}
