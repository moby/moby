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
		"Images:",
		"Execution Driver:",
		"Logging Driver:",
		"Operating System:",
		"CPUs:",
		"Total Memory:",
		"Kernel Version:",
		"Storage Driver:",
	}

	if utils.ExperimentalBuild() {
		stringsToCheck = append(stringsToCheck, "Experimental: true")
	}

	for _, linePrefix := range stringsToCheck {
		if !strings.Contains(out, linePrefix) {
			c.Errorf("couldn't find string %v in output", linePrefix)
		}
	}
}

// TestInfoDiscoveryBackend verifies that a daemon run with `--cluster-advertise` and
// `--cluster-store` properly show the backend's endpoint in info output.
func (s *DockerSuite) TestInfoDiscoveryBackend(c *check.C) {
	testRequires(c, SameHostDaemon)

	d := NewDaemon(c)
	discoveryBackend := "consul://consuladdr:consulport/some/path"
	discoveryAdvertise := "1.1.1.1:2375"
	err := d.Start(fmt.Sprintf("--cluster-store=%s", discoveryBackend), fmt.Sprintf("--cluster-advertise=%s", discoveryAdvertise))
	c.Assert(err, checker.IsNil)
	defer d.Stop()

	out, err := d.Cmd("info")
	c.Assert(err, checker.IsNil)
	c.Assert(out, checker.Contains, fmt.Sprintf("Cluster store: %s\n", discoveryBackend))
	c.Assert(out, checker.Contains, fmt.Sprintf("Cluster advertise: %s\n", discoveryAdvertise))
}

// TestInfoDiscoveryInvalidAdvertise verifies that a daemon run with
// an invalid `--cluster-advertise` configuration
func (s *DockerSuite) TestInfoDiscoveryInvalidAdvertise(c *check.C) {
	testRequires(c, SameHostDaemon)

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
	testRequires(c, SameHostDaemon)

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
	if len(addrs) <= 0 {
		c.Fatalf("addrs %v has to have at least one element", addrs)
	}
	ip, _, err := net.ParseCIDR(addrs[0].String())
	c.Assert(err, checker.IsNil)

	out, err := d.Cmd("info")
	c.Assert(err, checker.IsNil)
	c.Assert(out, checker.Contains, fmt.Sprintf("Cluster store: %s\n", discoveryBackend))
	c.Assert(out, checker.Contains, fmt.Sprintf("Cluster advertise: %s:2375\n", ip.String()))
}
