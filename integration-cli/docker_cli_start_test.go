package main

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/integration-cli/cli"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/icmd"
)

// Regression test for https://github.com/docker/docker/issues/7843
func (s *DockerSuite) TestStartAttachReturnsOnError(c *testing.T) {
	// Windows does not support link
	testRequires(c, DaemonIsLinux)
	dockerCmd(c, "run", "--name", "test", "busybox")

	// Expect this to fail because the above container is stopped, this is what we want
	out, _, err := dockerCmdWithError("run", "--name", "test2", "--link", "test:test", "busybox")
	// err shouldn't be nil because container test2 try to link to stopped container
	assert.Assert(c, err != nil, "out: %s", out)

	ch := make(chan error, 1)
	go func() {
		// Attempt to start attached to the container that won't start
		// This should return an error immediately since the container can't be started
		if out, _, err := dockerCmdWithError("start", "-a", "test2"); err == nil {
			ch <- fmt.Errorf("Expected error but got none:\n%s", out)
		}
		close(ch)
	}()

	select {
	case err := <-ch:
		assert.NilError(c, err)
	case <-time.After(5 * time.Second):
		c.Fatalf("Attach did not exit properly")
	}
}

// gh#8555: Exit code should be passed through when using start -a
func (s *DockerSuite) TestStartAttachCorrectExitCode(c *testing.T) {
	testRequires(c, DaemonIsLinux)
	out := cli.DockerCmd(c, "run", "-d", "busybox", "sh", "-c", "sleep 2; exit 1").Stdout()
	out = strings.TrimSpace(out)

	// make sure the container has exited before trying the "start -a"
	cli.DockerCmd(c, "wait", out)

	cli.Docker(cli.Args("start", "-a", out)).Assert(c, icmd.Expected{
		ExitCode: 1,
	})
}

func (s *DockerSuite) TestStartAttachSilent(c *testing.T) {
	name := "teststartattachcorrectexitcode"
	dockerCmd(c, "run", "--name", name, "busybox", "echo", "test")

	// make sure the container has exited before trying the "start -a"
	dockerCmd(c, "wait", name)

	startOut, _ := dockerCmd(c, "start", "-a", name)
	// start -a produced unexpected output
	assert.Equal(c, startOut, "test\n")
}

func (s *DockerSuite) TestStartRecordError(c *testing.T) {
	// TODO Windows CI: Requires further porting work. Should be possible.
	testRequires(c, DaemonIsLinux)
	// when container runs successfully, we should not have state.Error
	dockerCmd(c, "run", "-d", "-p", "9999:9999", "--name", "test", "busybox", "top")
	stateErr := inspectField(c, "test", "State.Error")
	// Expected to not have state error
	assert.Equal(c, stateErr, "")

	// Expect this to fail and records error because of ports conflict
	out, _, err := dockerCmdWithError("run", "-d", "--name", "test2", "-p", "9999:9999", "busybox", "top")
	// err shouldn't be nil because docker run will fail
	assert.Assert(c, err != nil, "out: %s", out)

	stateErr = inspectField(c, "test2", "State.Error")
	assert.Assert(c, strings.Contains(stateErr, "port is already allocated"))
	// Expect the conflict to be resolved when we stop the initial container
	dockerCmd(c, "stop", "test")
	dockerCmd(c, "start", "test2")
	stateErr = inspectField(c, "test2", "State.Error")
	// Expected to not have state error but got one
	assert.Equal(c, stateErr, "")
}

func (s *DockerSuite) TestStartPausedContainer(c *testing.T) {
	// Windows does not support pausing containers
	testRequires(c, IsPausable)

	runSleepingContainer(c, "-d", "--name", "testing")

	dockerCmd(c, "pause", "testing")

	out, _, err := dockerCmdWithError("start", "testing")
	// an error should have been shown that you cannot start paused container
	assert.Assert(c, err != nil, "out: %s", out)
	// an error should have been shown that you cannot start paused container
	assert.Assert(c, strings.Contains(strings.ToLower(out), "cannot start a paused container, try unpause instead"))
}

func (s *DockerSuite) TestStartMultipleContainers(c *testing.T) {
	// Windows does not support --link
	testRequires(c, DaemonIsLinux)
	// run a container named 'parent' and create two container link to `parent`
	dockerCmd(c, "run", "-d", "--name", "parent", "busybox", "top")

	for _, container := range []string{"child_first", "child_second"} {
		dockerCmd(c, "create", "--name", container, "--link", "parent:parent", "busybox", "top")
	}

	// stop 'parent' container
	dockerCmd(c, "stop", "parent")

	out := inspectField(c, "parent", "State.Running")
	// Container should be stopped
	assert.Equal(c, out, "false")

	// start all the three containers, container `child_first` start first which should be failed
	// container 'parent' start second and then start container 'child_second'
	expOut := "Cannot link to a non running container"
	expErr := "failed to start containers: [child_first]"
	out, _, err := dockerCmdWithError("start", "child_first", "parent", "child_second")
	// err shouldn't be nil because start will fail
	assert.Assert(c, err != nil, "out: %s", out)
	// output does not correspond to what was expected
	if !(strings.Contains(out, expOut) || strings.Contains(err.Error(), expErr)) {
		c.Fatalf("Expected out: %v with err: %v  but got out: %v with err: %v", expOut, expErr, out, err)
	}

	for container, expected := range map[string]string{"parent": "true", "child_first": "false", "child_second": "true"} {
		out := inspectField(c, container, "State.Running")
		// Container running state wrong
		assert.Equal(c, out, expected)
	}
}

func (s *DockerSuite) TestStartAttachMultipleContainers(c *testing.T) {
	// run  multiple containers to test
	for _, container := range []string{"test1", "test2", "test3"} {
		runSleepingContainer(c, "--name", container)
	}

	// stop all the containers
	for _, container := range []string{"test1", "test2", "test3"} {
		dockerCmd(c, "stop", container)
	}

	// test start and attach multiple containers at once, expected error
	for _, option := range []string{"-a", "-i", "-ai"} {
		out, _, err := dockerCmdWithError("start", option, "test1", "test2", "test3")
		// err shouldn't be nil because start will fail
		assert.Assert(c, err != nil, "out: %s", out)
		// output does not correspond to what was expected
		assert.Assert(c, strings.Contains(out, "you cannot start and attach multiple containers at once"))
	}

	// confirm the state of all the containers be stopped
	for container, expected := range map[string]string{"test1": "false", "test2": "false", "test3": "false"} {
		out := inspectField(c, container, "State.Running")
		// Container running state wrong
		assert.Equal(c, out, expected)
	}
}

// Test case for #23716
func (s *DockerSuite) TestStartAttachWithRename(c *testing.T) {
	testRequires(c, DaemonIsLinux)
	cli.DockerCmd(c, "create", "-t", "--name", "before", "busybox")
	go func() {
		cli.WaitRun(c, "before")
		cli.DockerCmd(c, "rename", "before", "after")
		cli.DockerCmd(c, "stop", "--time=2", "after")
	}()
	// FIXME(vdemeester) the intent is not clear and potentially racey
	result := cli.Docker(cli.Args("start", "-a", "before")).Assert(c, icmd.Expected{
		ExitCode: 137,
	})
	assert.Assert(c, !strings.Contains(result.Stderr(), "No such container"))
}

func (s *DockerSuite) TestStartReturnCorrectExitCode(c *testing.T) {
	cli.DockerCmd(c, "create", "--restart=on-failure:2", "--name", "withRestart", "busybox", "sh", "-c", "exit 11")
	cli.DockerCmd(c, "create", "--rm", "--name", "withRm", "busybox", "sh", "-c", "exit 12")
	cli.Docker(cli.Args("start", "-a", "withRestart")).Assert(c, icmd.Expected{ExitCode: 11})
	cli.Docker(cli.Args("start", "-a", "withRm")).Assert(c, icmd.Expected{ExitCode: 12})
}
