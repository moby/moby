//go:build !windows

package main

import (
	"testing"

	"github.com/moby/moby/v2/internal/testutil"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func (s *DockerSwarmSuite) TestServiceScale(c *testing.T) {
	ctx := testutil.GetContext(c)
	d := s.AddDaemon(ctx, c, true, true)

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
	assert.Check(c, is.Contains(out, service1Name))
	expected := "invalid replicas value"
	assert.Check(c, is.Contains(out, expected))

	out, err = d.Cmd("service", "scale", "TestService1=-1")
	assert.ErrorContains(c, err, "")
	assert.Check(c, is.Contains(out, service1Name))
	expected = "invalid replicas value"
	assert.Check(c, is.Contains(out, expected))

	// TestService2 is a global mode
	out, err = d.Cmd("service", "scale", "TestService2=2")
	assert.ErrorContains(c, err, "")
	assert.Check(c, is.Contains(out, service2Name))
	expected = "scale can only be used with replicated or replicated-job mode"
	assert.Check(c, is.Contains(out, expected))
}
