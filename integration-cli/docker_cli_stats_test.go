package main

import (
	"bufio"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/docker/docker/pkg/integration/checker"
	"github.com/go-check/check"
)

func (s *DockerSuite) TestStatsNoStream(c *check.C) {
	testRequires(c, DaemonIsLinux)
	out, _ := dockerCmd(c, "run", "-d", "busybox", "top")
	id := strings.TrimSpace(out)
	c.Assert(waitRun(id), checker.IsNil)

	statsCmd := exec.Command(dockerBinary, "stats", "--no-stream", id)
	type output struct {
		out []byte
		err error
	}

	ch := make(chan output)
	go func() {
		out, err := statsCmd.Output()
		ch <- output{out, err}
	}()

	select {
	case outerr := <-ch:
		c.Assert(outerr.err, checker.IsNil, check.Commentf("Error running stats: %v", outerr.err))
		c.Assert(string(outerr.out), checker.Contains, id) //running container wasn't present in output
	case <-time.After(3 * time.Second):
		statsCmd.Process.Kill()
		c.Fatalf("stats did not return immediately when not streaming")
	}
}

func (s *DockerSuite) TestStatsContainerNotFound(c *check.C) {
	testRequires(c, DaemonIsLinux)

	out, _, err := dockerCmdWithError("stats", "notfound")
	c.Assert(err, checker.NotNil)
	c.Assert(out, checker.Contains, "no such id: notfound", check.Commentf("Expected to fail on not found container stats, got %q instead", out))

	out, _, err = dockerCmdWithError("stats", "--no-stream", "notfound")
	c.Assert(err, checker.NotNil)
	c.Assert(out, checker.Contains, "no such id: notfound", check.Commentf("Expected to fail on not found container stats with --no-stream, got %q instead", out))
}

func (s *DockerSuite) TestStatsAllRunningNoStream(c *check.C) {
	testRequires(c, DaemonIsLinux)

	out, _ := dockerCmd(c, "run", "-d", "busybox", "top")
	id1 := strings.TrimSpace(out)[:12]
	c.Assert(waitRun(id1), check.IsNil)
	out, _ = dockerCmd(c, "run", "-d", "busybox", "top")
	id2 := strings.TrimSpace(out)[:12]
	c.Assert(waitRun(id2), check.IsNil)
	out, _ = dockerCmd(c, "run", "-d", "busybox", "top")
	id3 := strings.TrimSpace(out)[:12]
	c.Assert(waitRun(id3), check.IsNil)
	dockerCmd(c, "stop", id3)

	out, _ = dockerCmd(c, "stats", "--no-stream")
	if !strings.Contains(out, id1) || !strings.Contains(out, id2) {
		c.Fatalf("Expected stats output to contain both %s and %s, got %s", id1, id2, out)
	}
	if strings.Contains(out, id3) {
		c.Fatalf("Did not expect %s in stats, got %s", id3, out)
	}
}

func (s *DockerSuite) TestStatsAllNoStream(c *check.C) {
	testRequires(c, DaemonIsLinux)

	out, _ := dockerCmd(c, "run", "-d", "busybox", "top")
	id1 := strings.TrimSpace(out)[:12]
	c.Assert(waitRun(id1), check.IsNil)
	dockerCmd(c, "stop", id1)
	out, _ = dockerCmd(c, "run", "-d", "busybox", "top")
	id2 := strings.TrimSpace(out)[:12]
	c.Assert(waitRun(id2), check.IsNil)

	out, _ = dockerCmd(c, "stats", "--all", "--no-stream")
	if !strings.Contains(out, id1) || !strings.Contains(out, id2) {
		c.Fatalf("Expected stats output to contain both %s and %s, got %s", id1, id2, out)
	}
}

func (s *DockerSuite) TestStatsAllNewContainersAdded(c *check.C) {
	testRequires(c, DaemonIsLinux)

	id := make(chan string)
	addedChan := make(chan struct{})

	dockerCmd(c, "run", "-d", "busybox", "top")
	statsCmd := exec.Command(dockerBinary, "stats")
	stdout, err := statsCmd.StdoutPipe()
	c.Assert(err, check.IsNil)
	c.Assert(statsCmd.Start(), check.IsNil)
	defer statsCmd.Process.Kill()

	go func() {
		containerID := <-id
		matchID := regexp.MustCompile(containerID)

		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			switch {
			case matchID.MatchString(scanner.Text()):
				close(addedChan)
			}
		}
	}()

	out, _ := dockerCmd(c, "run", "-d", "busybox", "top")
	id <- strings.TrimSpace(out)[:12]

	select {
	case <-time.After(5 * time.Second):
		c.Fatal("failed to observe new container created added to stats")
	case <-addedChan:
		// ignore, done
	}
}
