package main

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/containerd/log"
	"github.com/docker/docker/integration-cli/cli"
	"github.com/docker/docker/integration-cli/daemon"
	"github.com/docker/docker/testutil"
	testdaemon "github.com/docker/docker/testutil/daemon"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/icmd"
)

type DockerCLILogsSuite struct {
	ds *DockerSuite
}

func (s *DockerCLILogsSuite) TearDownTest(ctx context.Context, c *testing.T) {
	s.ds.TearDownTest(ctx, c)
}

func (s *DockerCLILogsSuite) OnTimeout(c *testing.T) {
	s.ds.OnTimeout(c)
}

// This used to work, it test a log of PageSize-1 (gh#4851)
func (s *DockerCLILogsSuite) TestLogsContainerSmallerThanPage(c *testing.T) {
	testLogsContainerPagination(c, 32767)
}

// Regression test: When going over the PageSize, it used to panic (gh#4851)
func (s *DockerCLILogsSuite) TestLogsContainerBiggerThanPage(c *testing.T) {
	testLogsContainerPagination(c, 32768)
}

// Regression test: When going much over the PageSize, it used to block (gh#4851)
func (s *DockerCLILogsSuite) TestLogsContainerMuchBiggerThanPage(c *testing.T) {
	testLogsContainerPagination(c, 33000)
}

func testLogsContainerPagination(c *testing.T, testLen int) {
	id := cli.DockerCmd(c, "run", "-d", "busybox", "sh", "-c", fmt.Sprintf("for i in $(seq 1 %d); do echo -n = >> a.a; done; echo >> a.a; cat a.a", testLen)).Stdout()
	id = strings.TrimSpace(id)
	cli.DockerCmd(c, "wait", id)
	out := cli.DockerCmd(c, "logs", id).Combined()
	assert.Equal(c, len(out), testLen+1)
}

func (s *DockerCLILogsSuite) TestLogsTimestamps(c *testing.T) {
	testLen := 100
	id := cli.DockerCmd(c, "run", "-d", "busybox", "sh", "-c", fmt.Sprintf("for i in $(seq 1 %d); do echo = >> a.a; done; cat a.a", testLen)).Stdout()
	id = strings.TrimSpace(id)
	cli.DockerCmd(c, "wait", id)

	out := cli.DockerCmd(c, "logs", "-t", id).Combined()
	lines := strings.Split(out, "\n")

	assert.Equal(c, len(lines), testLen+1)

	ts := regexp.MustCompile(`^.* `)

	for _, l := range lines {
		if l != "" {
			_, err := time.Parse(log.RFC3339NanoFixed+" ", ts.FindString(l))
			assert.NilError(c, err, "Failed to parse timestamp from %v", l)
			// ensure we have padded 0's
			assert.Equal(c, l[29], uint8('Z'))
		}
	}
}

func (s *DockerCLILogsSuite) TestLogsSeparateStderr(c *testing.T) {
	msg := "stderr_log"
	out := cli.DockerCmd(c, "run", "-d", "busybox", "sh", "-c", fmt.Sprintf("echo %s 1>&2", msg)).Combined()
	id := strings.TrimSpace(out)
	cli.DockerCmd(c, "wait", id)
	cli.DockerCmd(c, "logs", id).Assert(c, icmd.Expected{
		Out: "",
		Err: msg,
	})
}

func (s *DockerCLILogsSuite) TestLogsStderrInStdout(c *testing.T) {
	// TODO Windows: Needs investigation why this fails. Obtained string includes
	// a bunch of ANSI escape sequences before the "stderr_log" message.
	testRequires(c, DaemonIsLinux)
	msg := "stderr_log"
	out := cli.DockerCmd(c, "run", "-d", "-t", "busybox", "sh", "-c", fmt.Sprintf("echo %s 1>&2", msg)).Combined()
	id := strings.TrimSpace(out)
	cli.DockerCmd(c, "wait", id)

	cli.DockerCmd(c, "logs", id).Assert(c, icmd.Expected{
		Out: msg,
		Err: "",
	})
}

func (s *DockerCLILogsSuite) TestLogsTail(c *testing.T) {
	testLen := 100
	out := cli.DockerCmd(c, "run", "-d", "busybox", "sh", "-c", fmt.Sprintf("for i in $(seq 1 %d); do echo =; done;", testLen)).Combined()

	id := strings.TrimSpace(out)
	cli.DockerCmd(c, "wait", id)

	out = cli.DockerCmd(c, "logs", "--tail", "0", id).Combined()
	lines := strings.Split(out, "\n")
	assert.Equal(c, len(lines), 1)

	out = cli.DockerCmd(c, "logs", "--tail", "5", id).Combined()
	lines = strings.Split(out, "\n")
	assert.Equal(c, len(lines), 6)

	out = cli.DockerCmd(c, "logs", "--tail", "99", id).Combined()
	lines = strings.Split(out, "\n")
	assert.Equal(c, len(lines), 100)

	out = cli.DockerCmd(c, "logs", "--tail", "all", id).Combined()
	lines = strings.Split(out, "\n")
	assert.Equal(c, len(lines), testLen+1)

	out = cli.DockerCmd(c, "logs", "--tail", "-1", id).Combined()
	lines = strings.Split(out, "\n")
	assert.Equal(c, len(lines), testLen+1)

	out = cli.DockerCmd(c, "logs", "--tail", "random", id).Combined()
	lines = strings.Split(out, "\n")
	assert.Equal(c, len(lines), testLen+1)
}

func (s *DockerCLILogsSuite) TestLogsFollowStopped(c *testing.T) {
	cli.DockerCmd(c, "run", "--name=test", "busybox", "echo", "hello")
	id := getIDByName(c, "test")

	logsCmd := exec.Command(dockerBinary, "logs", "-f", id)
	assert.NilError(c, logsCmd.Start())

	errChan := make(chan error, 1)
	go func() {
		errChan <- logsCmd.Wait()
		close(errChan)
	}()

	select {
	case err := <-errChan:
		assert.NilError(c, err)
	case <-time.After(30 * time.Second):
		c.Fatal("Following logs is hanged")
	}
}

func (s *DockerCLILogsSuite) TestLogsSince(c *testing.T) {
	name := "testlogssince"
	cli.DockerCmd(c, "run", "--name="+name, "busybox", "/bin/sh", "-c", "for i in $(seq 1 3); do sleep 2; echo log$i; done")
	out := cli.DockerCmd(c, "logs", "-t", name).Combined()

	log2Line := strings.Split(strings.Split(out, "\n")[1], " ")
	t, err := time.Parse(time.RFC3339Nano, log2Line[0]) // the timestamp log2 is written
	assert.NilError(c, err)
	since := t.Unix() + 1 // add 1s so log1 & log2 doesn't show up
	out = cli.DockerCmd(c, "logs", "-t", fmt.Sprintf("--since=%v", since), name).Combined()

	// Skip 2 seconds
	unexpected := []string{"log1", "log2"}
	for _, v := range unexpected {
		assert.Check(c, !strings.Contains(out, v), "unexpected log message returned, since=%v", since)
	}

	// Test to make sure a bad since format is caught by the client
	out, _, _ = dockerCmdWithError("logs", "-t", "--since=2006-01-02T15:04:0Z", name)
	assert.Assert(c, strings.Contains(out, `cannot parse "0Z" as "05"`), "bad since format passed to server")

	// Test with default value specified and parameter omitted
	expected := []string{"log1", "log2", "log3"}
	for _, cmd := range [][]string{
		{"logs", "-t", name},
		{"logs", "-t", "--since=0", name},
	} {
		result := icmd.RunCommand(dockerBinary, cmd...)
		result.Assert(c, icmd.Success)
		for _, v := range expected {
			assert.Check(c, strings.Contains(result.Combined(), v))
		}
	}
}

func (s *DockerCLILogsSuite) TestLogsSinceFutureFollow(c *testing.T) {
	// TODO Windows TP5 - Figure out why this test is so flakey. Disabled for now.
	testRequires(c, DaemonIsLinux)
	name := "testlogssincefuturefollow"
	cli.DockerCmd(c, "run", "-d", "--name", name, "busybox", "/bin/sh", "-c", `for i in $(seq 1 5); do echo log$i; sleep 1; done`)

	// Extract one timestamp from the log file to give us a starting point for
	// our `--since` argument. Because the log producer runs in the background,
	// we need to check repeatedly for some output to be produced.
	var timestamp string
	for i := 0; i != 100 && timestamp == ""; i++ {
		if out := cli.DockerCmd(c, "logs", "-t", name).Combined(); out == "" {
			time.Sleep(time.Millisecond * 100) // Retry
		} else {
			timestamp = strings.Split(strings.Split(out, "\n")[0], " ")[0]
		}
	}

	assert.Assert(c, timestamp != "")
	t, err := time.Parse(time.RFC3339Nano, timestamp)
	assert.NilError(c, err)

	since := t.Unix() + 2
	out := cli.DockerCmd(c, "logs", "-t", "-f", fmt.Sprintf("--since=%v", since), name).Combined()
	assert.Assert(c, len(out) != 0, "cannot read from empty log")
	lines := strings.Split(strings.TrimSpace(out), "\n")
	for _, v := range lines {
		ts, err := time.Parse(time.RFC3339Nano, strings.Split(v, " ")[0])
		assert.NilError(c, err, "cannot parse timestamp output from log: '%v'", v)
		assert.Assert(c, ts.Unix() >= since, "earlier log found. since=%v logdate=%v", since, ts)
	}
}

// Regression test for #8832
func (s *DockerCLILogsSuite) TestLogsFollowSlowStdoutConsumer(c *testing.T) {
	// TODO Windows: Fix this test for TP5.
	testRequires(c, DaemonIsLinux)
	expected := 150000
	id := cli.DockerCmd(c, "run", "-d", "busybox", "/bin/sh", "-c", fmt.Sprintf("usleep 600000; yes X | head -c %d", expected)).Stdout()
	id = strings.TrimSpace(id)

	stopSlowRead := make(chan bool)

	go func() {
		cli.DockerCmd(c, "wait", id)
		stopSlowRead <- true
	}()

	logCmd := exec.Command(dockerBinary, "logs", "-f", id)
	stdout, err := logCmd.StdoutPipe()
	assert.NilError(c, err)
	assert.NilError(c, logCmd.Start())
	defer func() { go logCmd.Wait() }()

	// First read slowly
	bytes1, err := ConsumeWithSpeed(stdout, 10, 50*time.Millisecond, stopSlowRead)
	assert.NilError(c, err)

	// After the container has finished we can continue reading fast
	bytes2, err := ConsumeWithSpeed(stdout, 32*1024, 0, nil)
	assert.NilError(c, err)

	assert.NilError(c, logCmd.Wait())

	actual := bytes1 + bytes2
	assert.Equal(c, actual, expected)
}

// ConsumeWithSpeed reads chunkSize bytes from reader before sleeping
// for interval duration. Returns total read bytes. Send true to the
// stop channel to return before reading to EOF on the reader.
func ConsumeWithSpeed(reader io.Reader, chunkSize int, interval time.Duration, stop chan bool) (n int, err error) {
	buffer := make([]byte, chunkSize)
	for {
		var readBytes int
		readBytes, err = reader.Read(buffer)
		n += readBytes
		if err != nil {
			if err == io.EOF {
				err = nil
			}
			return
		}
		select {
		case <-stop:
			return
		case <-time.After(interval):
		}
	}
}

func (s *DockerCLILogsSuite) TestLogsFollowGoroutinesWithStdout(c *testing.T) {
	testRequires(c, DaemonIsLinux, testEnv.IsLocalDaemon)
	c.Parallel()

	ctx := testutil.GetContext(c)
	d := daemon.New(c, dockerBinary, dockerdBinary, testdaemon.WithEnvVars("OTEL_SDK_DISABLED=1"))
	defer func() {
		d.Stop(c)
		d.Cleanup(c)
	}()
	d.StartWithBusybox(ctx, c, "--iptables=false")

	out, err := d.Cmd("run", "-d", "busybox", "/bin/sh", "-c", "while true; do echo hello; sleep 2; done")
	assert.NilError(c, err)

	id := strings.TrimSpace(out)
	assert.NilError(c, d.WaitRun(id))

	client := d.NewClientT(c)
	nroutines := waitForStableGourtineCount(ctx, c, client)

	cmd := d.Command("logs", "-f", id)
	r, w := io.Pipe()
	defer r.Close()
	defer w.Close()

	cmd.Stdout = w
	res := icmd.StartCmd(cmd)
	assert.NilError(c, res.Error)
	defer res.Cmd.Process.Kill()

	finished := make(chan error)
	go func() {
		finished <- res.Cmd.Wait()
	}()

	// Make sure pipe is written to
	chErr := make(chan error)
	go func() {
		b := make([]byte, 1)
		_, err := r.Read(b)
		chErr <- err
		r.Close()
	}()

	// Check read from pipe succeeded
	assert.NilError(c, <-chErr)

	assert.NilError(c, res.Cmd.Process.Kill())
	<-finished

	// NGoroutines is not updated right away, so we need to wait before failing
	waitForGoroutines(ctx, c, client, nroutines)
}

func (s *DockerCLILogsSuite) TestLogsFollowGoroutinesNoOutput(c *testing.T) {
	testRequires(c, DaemonIsLinux, testEnv.IsLocalDaemon)
	c.Parallel()

	d := daemon.New(c, dockerBinary, dockerdBinary, testdaemon.WithEnvVars("OTEL_SDK_DISABLED=1"))
	defer func() {
		d.Stop(c)
		d.Cleanup(c)
	}()

	ctx := testutil.GetContext(c)

	d.StartWithBusybox(ctx, c, "--iptables=false")

	out, err := d.Cmd("run", "-d", "busybox", "/bin/sh", "-c", "while true; do sleep 2; done")
	assert.NilError(c, err)
	id := strings.TrimSpace(out)
	assert.NilError(c, d.WaitRun(id))

	client := d.NewClientT(c)
	nroutines := waitForStableGourtineCount(ctx, c, client)
	assert.NilError(c, err)

	cmd := d.Command("logs", "-f", id)
	res := icmd.StartCmd(cmd)
	assert.NilError(c, res.Error)

	finished := make(chan error)
	go func() {
		finished <- res.Cmd.Wait()
	}()

	time.Sleep(200 * time.Millisecond)
	assert.NilError(c, res.Cmd.Process.Kill())

	<-finished

	// NGoroutines is not updated right away, so we need to wait before failing
	waitForGoroutines(ctx, c, client, nroutines)
}

func (s *DockerCLILogsSuite) TestLogsCLIContainerNotFound(c *testing.T) {
	name := "testlogsnocontainer"
	out, _, _ := dockerCmdWithError("logs", name)
	message := fmt.Sprintf("No such container: %s\n", name)
	assert.Assert(c, strings.Contains(out, message))
}

func (s *DockerCLILogsSuite) TestLogsWithDetails(c *testing.T) {
	cli.DockerCmd(c, "run", "--name=test", "--label", "foo=bar", "-e", "baz=qux", "--log-opt", "labels=foo", "--log-opt", "env=baz", "busybox", "echo", "hello")
	out := cli.DockerCmd(c, "logs", "--details", "--timestamps", "test").Combined()

	logFields := strings.Fields(strings.TrimSpace(out))
	assert.Equal(c, len(logFields), 3, out)

	details := strings.Split(logFields[1], ",")
	assert.Equal(c, len(details), 2)
	assert.Equal(c, details[0], "baz=qux")
	assert.Equal(c, details[1], "foo=bar")
}
