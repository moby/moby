package main

import (
	"bufio"
	"context"
	"os/exec"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/integration-cli/cli"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

type DockerCLIStatsSuite struct {
	ds *DockerSuite
}

func (s *DockerCLIStatsSuite) TearDownTest(ctx context.Context, c *testing.T) {
	s.ds.TearDownTest(ctx, c)
}

func (s *DockerCLIStatsSuite) OnTimeout(c *testing.T) {
	s.ds.OnTimeout(c)
}

func (s *DockerCLIStatsSuite) TestStatsNoStream(c *testing.T) {
	// Windows does not support stats
	testRequires(c, DaemonIsLinux)
	id := cli.DockerCmd(c, "run", "-d", "busybox", "top").Stdout()
	id = strings.TrimSpace(id)
	cli.WaitRun(c, id)

	statsCmd := exec.Command(dockerBinary, "stats", "--no-stream", id)
	type output struct {
		out []byte
		err error
	}

	ch := make(chan output, 1)
	go func() {
		out, err := statsCmd.Output()
		ch <- output{out, err}
	}()

	select {
	case outerr := <-ch:
		assert.NilError(c, outerr.err, "Error running stats: %v", outerr.err)
		assert.Assert(c, is.Contains(string(outerr.out), id[:12]), "running container wasn't present in output")
	case <-time.After(3 * time.Second):
		statsCmd.Process.Kill()
		c.Fatalf("stats did not return immediately when not streaming")
	}
}

func (s *DockerCLIStatsSuite) TestStatsContainerNotFound(c *testing.T) {
	// Windows does not support stats
	testRequires(c, DaemonIsLinux)

	out, _, err := dockerCmdWithError("stats", "notfound")
	assert.ErrorContains(c, err, "")
	assert.Assert(c, is.Contains(out, "No such container: notfound"), "Expected to fail on not found container stats, got %q instead", out)

	out, _, err = dockerCmdWithError("stats", "--no-stream", "notfound")
	assert.ErrorContains(c, err, "")
	assert.Assert(c, is.Contains(out, "No such container: notfound"), "Expected to fail on not found container stats with --no-stream, got %q instead", out)
}

func (s *DockerCLIStatsSuite) TestStatsAllRunningNoStream(c *testing.T) {
	// Windows does not support stats
	testRequires(c, DaemonIsLinux)

	id1 := cli.DockerCmd(c, "run", "-d", "busybox", "top").Stdout()
	id1 = strings.TrimSpace(id1)[:12]
	cli.WaitRun(c, id1)
	id2 := cli.DockerCmd(c, "run", "-d", "busybox", "top").Stdout()
	id2 = strings.TrimSpace(id2)[:12]
	cli.WaitRun(c, id2)
	id3 := cli.DockerCmd(c, "run", "-d", "busybox", "top").Stdout()
	id3 = strings.TrimSpace(id3)[:12]
	cli.WaitRun(c, id3)
	cli.DockerCmd(c, "stop", id3)

	out := cli.DockerCmd(c, "stats", "--no-stream").Combined()
	if !strings.Contains(out, id1) || !strings.Contains(out, id2) {
		c.Fatalf("Expected stats output to contain both %s and %s, got %s", id1, id2, out)
	}
	if strings.Contains(out, id3) {
		c.Fatalf("Did not expect %s in stats, got %s", id3, out)
	}

	// check output contains real data, but not all zeros
	reg, _ := regexp.Compile("[1-9]+")
	// split output with "\n", outLines[1] is id2's output
	// outLines[2] is id1's output
	outLines := strings.Split(out, "\n")
	// check stat result of id2 contains real data
	realData := reg.Find([]byte(outLines[1][12:]))
	assert.Assert(c, realData != nil, "stat result are empty: %s", out)
	// check stat result of id1 contains real data
	realData = reg.Find([]byte(outLines[2][12:]))
	assert.Assert(c, realData != nil, "stat result are empty: %s", out)
}

func (s *DockerCLIStatsSuite) TestStatsAllNoStream(c *testing.T) {
	// Windows does not support stats
	testRequires(c, DaemonIsLinux)

	id1 := cli.DockerCmd(c, "run", "-d", "busybox", "top").Stdout()
	id1 = strings.TrimSpace(id1)[:12]
	cli.WaitRun(c, id1)
	cli.DockerCmd(c, "stop", id1)
	id2 := cli.DockerCmd(c, "run", "-d", "busybox", "top").Stdout()
	id2 = strings.TrimSpace(id2)[:12]
	cli.WaitRun(c, id2)

	out := cli.DockerCmd(c, "stats", "--all", "--no-stream").Combined()
	if !strings.Contains(out, id1) || !strings.Contains(out, id2) {
		c.Fatalf("Expected stats output to contain both %s and %s, got %s", id1, id2, out)
	}

	// check output contains real data, but not all zeros
	reg, _ := regexp.Compile("[1-9]+")
	// split output with "\n", outLines[1] is id2's output
	outLines := strings.Split(out, "\n")
	// check stat result of id2 contains real data
	realData := reg.Find([]byte(outLines[1][12:]))
	assert.Assert(c, realData != nil, "stat result of %s is empty: %s", id2, out)

	// check stat result of id1 contains all zero
	realData = reg.Find([]byte(outLines[2][12:]))
	assert.Assert(c, realData == nil, "stat result of %s should be empty : %s", id1, out)
}

func (s *DockerCLIStatsSuite) TestStatsAllNewContainersAdded(c *testing.T) {
	// Windows does not support stats
	testRequires(c, DaemonIsLinux)

	id := make(chan string)
	addedChan := make(chan struct{})

	runSleepingContainer(c, "-d")
	statsCmd := exec.Command(dockerBinary, "stats")
	stdout, err := statsCmd.StdoutPipe()
	assert.NilError(c, err)
	assert.NilError(c, statsCmd.Start())
	go statsCmd.Wait()
	defer statsCmd.Process.Kill()

	go func() {
		containerID := <-id
		matchID := regexp.MustCompile(containerID)

		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			switch {
			case matchID.MatchString(scanner.Text()):
				close(addedChan)
				return
			}
		}
	}()

	out := runSleepingContainer(c, "-d")
	cli.WaitRun(c, out)
	id <- strings.TrimSpace(out)[:12]

	select {
	case <-time.After(30 * time.Second):
		c.Fatal("failed to observe new container created added to stats")
	case <-addedChan:
		// ignore, done
	}
}

func (s *DockerCLIStatsSuite) TestStatsFormatAll(c *testing.T) {
	// Windows does not support stats
	testRequires(c, DaemonIsLinux)

	cli.DockerCmd(c, "run", "-d", "--name=RunningOne", "busybox", "top")
	cli.WaitRun(c, "RunningOne")
	cli.DockerCmd(c, "run", "-d", "--name=ExitedOne", "busybox", "top")
	cli.DockerCmd(c, "stop", "ExitedOne")
	cli.WaitExited(c, "ExitedOne", 5*time.Second)

	out := cli.DockerCmd(c, "stats", "--no-stream", "--format", "{{.Name}}").Combined()
	assert.Assert(c, is.Contains(out, "RunningOne"))
	assert.Assert(c, !strings.Contains(out, "ExitedOne"))

	out = cli.DockerCmd(c, "stats", "--all", "--no-stream", "--format", "{{.Name}}").Combined()
	assert.Assert(c, is.Contains(out, "RunningOne"))
	assert.Assert(c, is.Contains(out, "ExitedOne"))
}
