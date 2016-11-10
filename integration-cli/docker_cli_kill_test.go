package main

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/docker/docker/pkg/integration/checker"
	"github.com/go-check/check"
)

func (s *DockerSuite) TestKillContainer(c *check.C) {
	out, _ := runSleepingContainer(c, "-d")
	cleanedContainerID := strings.TrimSpace(out)
	c.Assert(waitRun(cleanedContainerID), check.IsNil)

	dockerCmd(c, "kill", cleanedContainerID)
	c.Assert(waitExited(cleanedContainerID, 10*time.Second), check.IsNil)

	out, _ = dockerCmd(c, "ps", "-q")
	c.Assert(out, checker.Not(checker.Contains), cleanedContainerID, check.Commentf("killed container is still running"))

}

func (s *DockerSuite) TestKillOffStoppedContainer(c *check.C) {
	out, _ := runSleepingContainer(c, "-d")
	cleanedContainerID := strings.TrimSpace(out)

	dockerCmd(c, "stop", cleanedContainerID)
	c.Assert(waitExited(cleanedContainerID, 10*time.Second), check.IsNil)

	_, _, err := dockerCmdWithError("kill", "-s", "30", cleanedContainerID)
	c.Assert(err, check.Not(check.IsNil), check.Commentf("Container %s is not running", cleanedContainerID))
}

func (s *DockerSuite) TestKillDifferentUserContainer(c *check.C) {
	// TODO Windows: Windows does not yet support -u (Feb 2016).
	testRequires(c, DaemonIsLinux)
	out, _ := dockerCmd(c, "run", "-u", "daemon", "-d", "busybox", "top")
	cleanedContainerID := strings.TrimSpace(out)
	c.Assert(waitRun(cleanedContainerID), check.IsNil)

	dockerCmd(c, "kill", cleanedContainerID)
	c.Assert(waitExited(cleanedContainerID, 10*time.Second), check.IsNil)

	out, _ = dockerCmd(c, "ps", "-q")
	c.Assert(out, checker.Not(checker.Contains), cleanedContainerID, check.Commentf("killed container is still running"))

}

// regression test about correct signal parsing see #13665
func (s *DockerSuite) TestKillWithSignal(c *check.C) {
	// Cannot port to Windows - does not support signals in the same way Linux does
	testRequires(c, DaemonIsLinux)
	out, _ := dockerCmd(c, "run", "-d", "busybox", "top")
	cid := strings.TrimSpace(out)
	c.Assert(waitRun(cid), check.IsNil)

	dockerCmd(c, "kill", "-s", "SIGWINCH", cid)
	time.Sleep(250 * time.Millisecond)

	running := inspectField(c, cid, "State.Running")

	c.Assert(running, checker.Equals, "true", check.Commentf("Container should be in running state after SIGWINCH"))
}

func (s *DockerSuite) TestKillWithStopSignalWithSameSignalShouldDisableRestartPolicy(c *check.C) {
	// Cannot port to Windows - does not support signals int the same way as Linux does
	testRequires(c, DaemonIsLinux)
	out, _ := dockerCmd(c, "run", "-d", "--stop-signal=TERM", "--restart=always", "busybox", "top")
	cid := strings.TrimSpace(out)
	c.Assert(waitRun(cid), check.IsNil)

	// Let docker send a TERM signal to the container
	// It will kill the process and disable the restart policy
	dockerCmd(c, "kill", "-s", "TERM", cid)
	c.Assert(waitExited(cid, 10*time.Second), check.IsNil)

	out, _ = dockerCmd(c, "ps", "-q")
	c.Assert(out, checker.Not(checker.Contains), cid, check.Commentf("killed container is still running"))
}

func (s *DockerSuite) TestKillWithStopSignalWithDifferentSignalShouldKeepRestartPolicy(c *check.C) {
	// Cannot port to Windows - does not support signals int the same way as Linux does
	testRequires(c, DaemonIsLinux)
	out, _ := dockerCmd(c, "run", "-d", "--stop-signal=CONT", "--restart=always", "busybox", "top")
	cid := strings.TrimSpace(out)
	c.Assert(waitRun(cid), check.IsNil)

	// Let docker send a TERM signal to the container
	// It will kill the process, but not disable the restart policy
	dockerCmd(c, "kill", "-s", "TERM", cid)
	c.Assert(waitRestart(cid, 10*time.Second), check.IsNil)

	// Restart policy should still be in place, so it should be still running
	c.Assert(waitRun(cid), check.IsNil)
}

// FIXME(vdemeester) should be a unit test
func (s *DockerSuite) TestKillWithInvalidSignal(c *check.C) {
	out, _ := runSleepingContainer(c, "-d")
	cid := strings.TrimSpace(out)
	c.Assert(waitRun(cid), check.IsNil)

	out, _, err := dockerCmdWithError("kill", "-s", "0", cid)
	c.Assert(err, check.NotNil)
	c.Assert(out, checker.Contains, "Invalid signal: 0", check.Commentf("Kill with an invalid signal didn't error out correctly"))

	running := inspectField(c, cid, "State.Running")
	c.Assert(running, checker.Equals, "true", check.Commentf("Container should be in running state after an invalid signal"))

	out, _ = runSleepingContainer(c, "-d")
	cid = strings.TrimSpace(out)
	c.Assert(waitRun(cid), check.IsNil)

	out, _, err = dockerCmdWithError("kill", "-s", "SIG42", cid)
	c.Assert(err, check.NotNil)
	c.Assert(out, checker.Contains, "Invalid signal: SIG42", check.Commentf("Kill with an invalid signal error out correctly"))

	running = inspectField(c, cid, "State.Running")
	c.Assert(running, checker.Equals, "true", check.Commentf("Container should be in running state after an invalid signal"))

}

func (s *DockerSuite) TestKillStoppedContainerAPIPre120(c *check.C) {
	testRequires(c, DaemonIsLinux) // Windows only supports 1.25 or later
	runSleepingContainer(c, "--name", "docker-kill-test-api", "-d")
	dockerCmd(c, "stop", "docker-kill-test-api")

	status, _, err := sockRequest("POST", fmt.Sprintf("/v1.19/containers/%s/kill", "docker-kill-test-api"), nil)
	c.Assert(err, check.IsNil)
	c.Assert(status, check.Equals, http.StatusNoContent)
}
