//go:build !windows
// +build !windows

package main

import (
	"fmt"
	"strings"
	"testing"

	"gotest.tools/v3/assert"
)

func (s *DockerSwarmSuite) TestServiceScale(c *testing.T) {
	d := s.AddDaemon(c, true, true)

	service1Name := "TestService1"
	service1Args := append([]string{"service", "create", "--detach", "--no-resolve-image", "--name", service1Name, "busybox"}, sleepCommandForDaemonPlatform()...)

	// global mode
	service2Name := "TestService2"
	service2Args := append([]string{"service", "create", "--detach", "--no-resolve-image", "--name", service2Name, "--mode=global", "busybox"}, sleepCommandForDaemonPlatform()...)

	// Create services
	_, err := d.Cmd(service1Args...)
	assert.NilError(c, err)

	_, err = d.Cmd(service2Args...)
	assert.NilError(c, err)

	_, err = d.Cmd("service", "scale", "TestService1=2")
	assert.NilError(c, err)

	out, err := d.Cmd("service", "scale", "TestService1=foobar")
	assert.ErrorContains(c, err, "")

	str := fmt.Sprintf("%s: invalid replicas value %s", service1Name, "foobar")
	if !strings.Contains(out, str) {
		c.Errorf("got: %s, expected has sub string: %s", out, str)
	}

	out, err = d.Cmd("service", "scale", "TestService1=-1")
	assert.ErrorContains(c, err, "")

	str = fmt.Sprintf("%s: invalid replicas value %s", service1Name, "-1")
	if !strings.Contains(out, str) {
		c.Errorf("got: %s, expected has sub string: %s", out, str)
	}

	// TestService2 is a global mode
	out, err = d.Cmd("service", "scale", "TestService2=2")
	assert.ErrorContains(c, err, "")

	str = fmt.Sprintf("%s: scale can only be used with replicated mode\n", service2Name)
	if out != str {
		c.Errorf("got: %s, expected: %s", out, str)
	}
}
