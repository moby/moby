package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
	eventtypes "github.com/docker/docker/api/types/events"
	"github.com/docker/docker/client"
	eventstestutils "github.com/docker/docker/daemon/events/testutils"
	"github.com/docker/docker/integration-cli/cli"
	"github.com/docker/docker/integration-cli/cli/build"
	"github.com/docker/docker/testutil"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/icmd"
)

type DockerCLIEventSuite struct {
	ds *DockerSuite
}

func (s *DockerCLIEventSuite) TearDownTest(ctx context.Context, c *testing.T) {
	s.ds.TearDownTest(ctx, c)
}

func (s *DockerCLIEventSuite) OnTimeout(c *testing.T) {
	s.ds.OnTimeout(c)
}

func (s *DockerCLIEventSuite) TestEventsTimestampFormats(c *testing.T) {
	name := "events-time-format-test"

	// Start stopwatch, generate an event
	start := daemonTime(c)
	time.Sleep(1100 * time.Millisecond) // so that first event occur in different second from since (just for the case)
	cli.DockerCmd(c, "run", "--rm", "--name", name, "busybox", "true")
	time.Sleep(1100 * time.Millisecond) // so that until > since
	end := daemonTime(c)

	// List of available time formats to --since
	unixTs := func(t time.Time) string { return fmt.Sprintf("%v", t.Unix()) }
	rfc3339 := func(t time.Time) string { return t.Format(time.RFC3339) }
	duration := func(t time.Time) string { return time.Since(t).String() }

	// --since=$start must contain only the 'untag' event
	for _, f := range []func(time.Time) string{unixTs, rfc3339, duration} {
		since, until := f(start), f(end)
		out := cli.DockerCmd(c, "events", "--since="+since, "--until="+until).Stdout()
		events := strings.Split(out, "\n")
		events = events[:len(events)-1]

		nEvents := len(events)
		assert.Assert(c, nEvents >= 5)
		containerEvents := eventActionsByIDAndType(c, events, name, "container")
		assert.Assert(c, is.DeepEqual(containerEvents, []string{"create", "attach", "start", "die", "destroy"}), out)
	}
}

func (s *DockerCLIEventSuite) TestEventsUntag(c *testing.T) {
	const imgName = "busybox"
	cli.DockerCmd(c, "tag", imgName, "utest:tag1")
	cli.DockerCmd(c, "tag", imgName, "utest:tag2")
	cli.DockerCmd(c, "rmi", "utest:tag1")
	cli.DockerCmd(c, "rmi", "utest:tag2")

	result := icmd.RunCmd(icmd.Cmd{
		Command: []string{dockerBinary, "events", "--since=1"},
		Timeout: time.Millisecond * 2500,
	})
	result.Assert(c, icmd.Expected{Timeout: true})

	events := strings.Split(result.Stdout(), "\n")
	nEvents := len(events)
	// The last element after the split above will be an empty string, so we
	// get the two elements before the last, which are the untags we're
	// looking for.
	for _, v := range events[nEvents-3 : nEvents-1] {
		assert.Check(c, strings.Contains(v, "untag"), "event should be untag")
	}
}

func (s *DockerCLIEventSuite) TestEventsContainerEvents(c *testing.T) {
	cli.DockerCmd(c, "run", "--rm", "--name", "container-events-test", "busybox", "true")

	out := cli.DockerCmd(c, "events", "--until", daemonUnixTime(c)).Stdout()
	events := strings.Split(out, "\n")
	events = events[:len(events)-1]

	containerEvents := eventActionsByIDAndType(c, events, "container-events-test", "container")
	if len(containerEvents) > 5 {
		containerEvents = containerEvents[:5]
	}
	assert.Assert(c, is.DeepEqual(containerEvents, []string{"create", "attach", "start", "die", "destroy"}), out)
}

func (s *DockerCLIEventSuite) TestEventsContainerEventsAttrSort(c *testing.T) {
	since := daemonUnixTime(c)
	cli.DockerCmd(c, "run", "--rm", "--name", "container-events-test", "busybox", "true")

	out := cli.DockerCmd(c, "events", "--filter", "container=container-events-test", "--since", since, "--until", daemonUnixTime(c)).Stdout()
	events := strings.Split(out, "\n")

	nEvents := len(events)
	assert.Assert(c, nEvents >= 3)
	matchedEvents := 0
	for _, event := range events {
		matches := eventstestutils.ScanMap(event)
		if matches["eventType"] == "container" && matches["action"] == "create" {
			matchedEvents++
			assert.Check(c, strings.Contains(out, "(image=busybox, name=container-events-test)"), "Event attributes not sorted")
		} else if matches["eventType"] == "container" && matches["action"] == "start" {
			matchedEvents++
			assert.Check(c, strings.Contains(out, "(image=busybox, name=container-events-test)"), "Event attributes not sorted")
		}
	}
	assert.Equal(c, matchedEvents, 2, "missing events for container container-events-test:\n%s", out)
}

func (s *DockerCLIEventSuite) TestEventsContainerEventsSinceUnixEpoch(c *testing.T) {
	cli.DockerCmd(c, "run", "--rm", "--name", "since-epoch-test", "busybox", "true")
	timeBeginning := time.Unix(0, 0).Format(time.RFC3339Nano)
	timeBeginning = strings.ReplaceAll(timeBeginning, "Z", ".000000000Z")
	out := cli.DockerCmd(c, "events", "--since", timeBeginning, "--until", daemonUnixTime(c)).Stdout()
	events := strings.Split(out, "\n")
	events = events[:len(events)-1]

	nEvents := len(events)
	assert.Assert(c, nEvents >= 5)
	containerEvents := eventActionsByIDAndType(c, events, "since-epoch-test", "container")
	assert.Assert(c, is.DeepEqual(containerEvents, []string{"create", "attach", "start", "die", "destroy"}), out)
}

func (s *DockerCLIEventSuite) TestEventsImageTag(c *testing.T) {
	time.Sleep(1 * time.Second) // because API has seconds granularity
	since := daemonUnixTime(c)
	const imgName = "testimageevents:tag"
	cli.DockerCmd(c, "tag", "busybox", imgName)

	out := cli.DockerCmd(c, "events", "--since", since, "--until", daemonUnixTime(c)).Stdout()

	events := strings.Split(strings.TrimSpace(out), "\n")
	assert.Equal(c, len(events), 1, "was expecting 1 event. out=%s", out)
	event := strings.TrimSpace(events[0])

	matches := eventstestutils.ScanMap(event)
	assert.Assert(c, matchEventID(matches, imgName), "matches: %v\nout:\n%s", matches, out)
	assert.Equal(c, matches["action"], "tag")
}

func (s *DockerCLIEventSuite) TestEventsImagePull(c *testing.T) {
	// TODO Windows: Enable this test once pull and reliable image names are available
	testRequires(c, DaemonIsLinux)
	since := daemonUnixTime(c)
	testRequires(c, Network)

	cli.DockerCmd(c, "pull", "hello-world")

	out := cli.DockerCmd(c, "events", "--since", since, "--until", daemonUnixTime(c)).Stdout()
	events := strings.Split(strings.TrimSpace(out), "\n")
	event := strings.TrimSpace(events[len(events)-1])
	matches := eventstestutils.ScanMap(event)
	assert.Equal(c, matches["id"], "hello-world:latest")
	assert.Equal(c, matches["action"], "pull")
}

func (s *DockerCLIEventSuite) TestEventsImageImport(c *testing.T) {
	// TODO Windows CI. This should be portable once export/import are
	// more reliable (@swernli)
	testRequires(c, DaemonIsLinux)

	out := cli.DockerCmd(c, "run", "-d", "busybox", "true").Stdout()
	cleanedContainerID := strings.TrimSpace(out)

	since := daemonUnixTime(c)
	out, err := RunCommandPipelineWithOutput(
		exec.Command(dockerBinary, "export", cleanedContainerID),
		exec.Command(dockerBinary, "import", "-"),
	)
	assert.NilError(c, err, "import failed with output: %q", out)
	imageRef := strings.TrimSpace(out)

	out = cli.DockerCmd(c, "events", "--since", since, "--until", daemonUnixTime(c), "--filter", "event=import").Stdout()
	events := strings.Split(strings.TrimSpace(out), "\n")
	assert.Equal(c, len(events), 1)
	matches := eventstestutils.ScanMap(events[0])
	assert.Equal(c, matches["id"], imageRef, "matches: %v\nout:\n%s\n", matches, out)
	assert.Equal(c, matches["action"], "import", "matches: %v\nout:\n%s\n", matches, out)
}

func (s *DockerCLIEventSuite) TestEventsImageLoad(c *testing.T) {
	testRequires(c, DaemonIsLinux)
	myImageName := "footest:v1"
	cli.DockerCmd(c, "tag", "busybox", myImageName)
	since := daemonUnixTime(c)

	out := cli.DockerCmd(c, "images", "-q", "--no-trunc", myImageName).Stdout()
	longImageID := strings.TrimSpace(out)
	assert.Assert(c, longImageID != "", "Id should not be empty")

	cli.DockerCmd(c, "save", "-o", "saveimg.tar", myImageName)
	cli.DockerCmd(c, "rmi", myImageName)
	out = cli.DockerCmd(c, "images", "-q", myImageName).Stdout()
	noImageID := strings.TrimSpace(out)
	assert.Equal(c, noImageID, "", "Should not have any image")
	cli.DockerCmd(c, "load", "-i", "saveimg.tar")

	result := icmd.RunCommand("rm", "-rf", "saveimg.tar")
	result.Assert(c, icmd.Success)

	out = cli.DockerCmd(c, "images", "-q", "--no-trunc", myImageName).Stdout()
	imageID := strings.TrimSpace(out)
	assert.Equal(c, imageID, longImageID, "Should have same image id as before")

	out = cli.DockerCmd(c, "events", "--since", since, "--until", daemonUnixTime(c), "--filter", "event=load").Stdout()
	events := strings.Split(strings.TrimSpace(out), "\n")
	assert.Equal(c, len(events), 1)
	matches := eventstestutils.ScanMap(events[0])
	assert.Equal(c, matches["id"], imageID, "matches: %v\nout:\n%s\n", matches, out)
	assert.Equal(c, matches["action"], "load", "matches: %v\nout:\n%s\n", matches, out)

	out = cli.DockerCmd(c, "events", "--since", since, "--until", daemonUnixTime(c), "--filter", "event=save").Stdout()
	events = strings.Split(strings.TrimSpace(out), "\n")
	assert.Equal(c, len(events), 1)
	matches = eventstestutils.ScanMap(events[0])
	assert.Equal(c, matches["id"], imageID, "matches: %v\nout:\n%s\n", matches, out)
	assert.Equal(c, matches["action"], "save", "matches: %v\nout:\n%s\n", matches, out)
}

func (s *DockerCLIEventSuite) TestEventsPluginOps(c *testing.T) {
	testRequires(c, DaemonIsLinux, IsAmd64, Network)

	since := daemonUnixTime(c)

	cli.DockerCmd(c, "plugin", "install", pNameWithTag, "--grant-all-permissions")
	cli.DockerCmd(c, "plugin", "disable", pNameWithTag)
	cli.DockerCmd(c, "plugin", "remove", pNameWithTag)

	out := cli.DockerCmd(c, "events", "--since", since, "--until", daemonUnixTime(c)).Stdout()
	events := strings.Split(out, "\n")
	events = events[:len(events)-1]

	assert.Assert(c, len(events) >= 4)

	pluginEvents := eventActionsByIDAndType(c, events, pNameWithTag, "plugin")
	assert.Assert(c, is.DeepEqual(pluginEvents, []string{"pull", "enable", "disable", "remove"}), out)
}

func (s *DockerCLIEventSuite) TestEventsFilters(c *testing.T) {
	since := daemonUnixTime(c)
	cli.DockerCmd(c, "run", "--rm", "busybox", "true")
	cli.DockerCmd(c, "run", "--rm", "busybox", "true")
	out := cli.DockerCmd(c, "events", "--since", since, "--until", daemonUnixTime(c), "--filter", "event=die").Stdout()
	parseEvents(c, out, "die")

	out = cli.DockerCmd(c, "events", "--since", since, "--until", daemonUnixTime(c), "--filter", "event=die", "--filter", "event=start").Stdout()
	parseEvents(c, out, "die|start")

	// make sure we at least got 2 start events
	count := strings.Count(out, "start")
	assert.Assert(c, count >= 2, "should have had 2 start events but had %d, out: %s", count, out)
}

func (s *DockerCLIEventSuite) TestEventsFilterImageName(c *testing.T) {
	since := daemonUnixTime(c)

	out := cli.DockerCmd(c, "run", "--name", "container_1", "-d", "busybox:latest", "true").Stdout()
	container1 := strings.TrimSpace(out)

	out = cli.DockerCmd(c, "run", "--name", "container_2", "-d", "busybox", "true").Stdout()
	container2 := strings.TrimSpace(out)

	name := "busybox"
	out = cli.DockerCmd(c, "events", "--since", since, "--until", daemonUnixTime(c), "--filter", fmt.Sprintf("image=%s", name)).Stdout()
	events := strings.Split(out, "\n")
	events = events[:len(events)-1]
	assert.Assert(c, len(events) != 0, "Expected events but found none for the image busybox:latest")
	count1 := 0
	count2 := 0

	for _, e := range events {
		if strings.Contains(e, container1) {
			count1++
		} else if strings.Contains(e, container2) {
			count2++
		}
	}
	assert.Assert(c, count1 != 0, "Expected event from container but got %d from %s", count1, container1)
	assert.Assert(c, count2 != 0, "Expected event from container but got %d from %s", count2, container2)
}

func (s *DockerCLIEventSuite) TestEventsFilterLabels(c *testing.T) {
	since := strconv.FormatUint(uint64(daemonTime(c).Unix()), 10)
	label := "io.docker.testing=foo"

	result := cli.DockerCmd(c, "create", "-l", label, "busybox")
	assert.Equal(c, result.ExitCode, 0)
	container1 := strings.TrimSpace(result.Stdout())

	result = cli.DockerCmd(c, "create", "busybox")
	assert.Equal(c, result.ExitCode, 0)
	container2 := strings.TrimSpace(result.Stdout())

	// fetch events with `--until`, so that the client detaches after a second
	// instead of staying attached, waiting for more events to arrive.
	out := cli.DockerCmd(c,
		"events",
		"--since", since,
		"--until", strconv.FormatUint(uint64(daemonTime(c).Add(time.Second).Unix()), 10),
		"--filter", "label="+label,
	).Stdout()

	events := strings.Split(strings.TrimSpace(out), "\n")
	assert.Assert(c, len(events) > 0)

	var found bool
	for _, e := range events {
		if strings.Contains(e, container1) {
			found = true
		}
		assert.Assert(c, !strings.Contains(e, container2))
	}
	assert.Assert(c, found)
}

func (s *DockerCLIEventSuite) TestEventsFilterImageLabels(c *testing.T) {
	since := daemonUnixTime(c)
	name := "labelfiltertest"
	label := "io.docker.testing=image"

	// Build a test image.
	buildImageSuccessfully(c, name, build.WithDockerfile(fmt.Sprintf(`
		FROM busybox:latest
		LABEL %s`, label)))
	cli.DockerCmd(c, "tag", name, "labelfiltertest:tag1")
	cli.DockerCmd(c, "tag", name, "labelfiltertest:tag2")
	cli.DockerCmd(c, "tag", "busybox:latest", "labelfiltertest:tag3")

	out := cli.DockerCmd(c,
		"events",
		"--since", since,
		"--until", daemonUnixTime(c),
		"--filter", fmt.Sprintf("label=%s", label),
		"--filter", "type=image",
	).Stdout()

	events := strings.Split(strings.TrimSpace(out), "\n")

	// 2 events from the "docker tag" command, another one is from "docker build"
	assert.Equal(c, len(events), 3, "Events == %s", events)
	for _, e := range events {
		assert.Check(c, strings.Contains(e, "labelfiltertest"))
	}
}

func (s *DockerCLIEventSuite) TestEventsFilterContainer(c *testing.T) {
	since := daemonUnixTime(c)
	nameID := make(map[string]string)

	for _, name := range []string{"container_1", "container_2"} {
		cli.DockerCmd(c, "run", "--name", name, "busybox", "true")
		id := inspectField(c, name, "Id")
		nameID[name] = id
	}

	until := daemonUnixTime(c)

	checkEvents := func(id string, events []string) error {
		if len(events) != 4 { // create, attach, start, die
			return fmt.Errorf("expected 4 events, got %v", events)
		}
		for _, event := range events {
			matches := eventstestutils.ScanMap(event)
			if !matchEventID(matches, id) {
				return fmt.Errorf("expected event for container id %s: %s - parsed container id: %s", id, event, matches["id"])
			}
		}
		return nil
	}

	for name, ID := range nameID {
		// filter by names
		out := cli.DockerCmd(c, "events", "--since", since, "--until", until, "--filter", "container="+name).Stdout()
		events := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
		assert.NilError(c, checkEvents(ID, events))

		// filter by ID's
		out = cli.DockerCmd(c, "events", "--since", since, "--until", until, "--filter", "container="+ID).Stdout()
		events = strings.Split(strings.TrimSuffix(out, "\n"), "\n")
		assert.NilError(c, checkEvents(ID, events))
	}
}

func (s *DockerCLIEventSuite) TestEventsCommit(c *testing.T) {
	// Problematic on Windows as cannot commit a running container
	testRequires(c, DaemonIsLinux)

	cID := runSleepingContainer(c)
	cli.WaitRun(c, cID)

	cli.DockerCmd(c, "commit", "-m", "test", cID)
	cli.DockerCmd(c, "stop", cID)
	cli.WaitExited(c, cID, 5*time.Second)

	until := daemonUnixTime(c)
	out := cli.DockerCmd(c, "events", "-f", "container="+cID, "--until="+until).Combined()
	assert.Assert(c, strings.Contains(out, "commit"), "Missing 'commit' log event")
}

func (s *DockerCLIEventSuite) TestEventsCopy(c *testing.T) {
	// Build a test image.
	buildImageSuccessfully(c, "cpimg", build.WithDockerfile(`
		  FROM busybox
		  RUN echo HI > /file`))
	id := getIDByName(c, "cpimg")

	// Create an empty test file.
	tempFile, err := os.CreateTemp("", "test-events-copy-")
	assert.NilError(c, err)
	defer os.Remove(tempFile.Name())

	assert.NilError(c, tempFile.Close())

	cli.DockerCmd(c, "create", "--name=cptest", id)

	cli.DockerCmd(c, "cp", "cptest:/file", tempFile.Name())

	until := daemonUnixTime(c)
	out := cli.DockerCmd(c, "events", "--since=0", "-f", "container=cptest", "--until="+until).Stdout()
	assert.Assert(c, strings.Contains(out, "archive-path"), "Missing 'archive-path' log event")

	cli.DockerCmd(c, "cp", tempFile.Name(), "cptest:/filecopy")

	until = daemonUnixTime(c)
	out = cli.DockerCmd(c, "events", "-f", "container=cptest", "--until="+until).Stdout()
	assert.Assert(c, strings.Contains(out, "extract-to-dir"), "Missing 'extract-to-dir' log event")
}

func (s *DockerCLIEventSuite) TestEventsResize(c *testing.T) {
	cID := runSleepingContainer(c, "-d", "-t")
	cli.WaitRun(c, cID)

	apiClient, err := client.NewClientWithOpts(client.FromEnv)
	assert.NilError(c, err)
	defer apiClient.Close()

	options := container.ResizeOptions{
		Height: 80,
		Width:  24,
	}
	err = apiClient.ContainerResize(testutil.GetContext(c), cID, options)
	assert.NilError(c, err)

	cli.DockerCmd(c, "stop", cID)

	until := daemonUnixTime(c)
	out := cli.DockerCmd(c, "events", "-f", "container="+cID, "--until="+until).Combined()
	assert.Assert(c, strings.Contains(out, "resize"), "Missing 'resize' log event")
}

func (s *DockerCLIEventSuite) TestEventsAttach(c *testing.T) {
	// TODO Windows CI: Figure out why this test fails intermittently (TP5).
	testRequires(c, DaemonIsLinux)

	out := cli.DockerCmd(c, "run", "-di", "busybox", "cat").Combined()
	cID := strings.TrimSpace(out)
	cli.WaitRun(c, cID)

	cmd := exec.Command(dockerBinary, "attach", cID)
	stdin, err := cmd.StdinPipe()
	assert.NilError(c, err)
	defer stdin.Close()
	stdout, err := cmd.StdoutPipe()
	assert.NilError(c, err)
	defer stdout.Close()
	assert.NilError(c, cmd.Start())
	defer func() {
		cmd.Process.Kill()
		cmd.Wait()
	}()

	// Make sure we're done attaching by writing/reading some stuff
	_, err = stdin.Write([]byte("hello\n"))
	assert.NilError(c, err)
	out, err = bufio.NewReader(stdout).ReadString('\n')
	assert.NilError(c, err)
	assert.Equal(c, strings.TrimSpace(out), "hello")

	assert.NilError(c, stdin.Close())

	cli.DockerCmd(c, "kill", cID)
	cli.WaitExited(c, cID, 5*time.Second)

	until := daemonUnixTime(c)
	out = cli.DockerCmd(c, "events", "-f", "container="+cID, "--until="+until).Combined()
	assert.Assert(c, strings.Contains(out, "attach"), "Missing 'attach' log event")
}

func (s *DockerCLIEventSuite) TestEventsRename(c *testing.T) {
	out := cli.DockerCmd(c, "run", "--name", "oldName", "busybox", "true").Stdout()
	cID := strings.TrimSpace(out)
	cli.DockerCmd(c, "rename", "oldName", "newName")

	until := daemonUnixTime(c)
	// filter by the container id because the name in the event will be the new name.
	out = cli.DockerCmd(c, "events", "-f", "container="+cID, "--until", until).Stdout()
	assert.Assert(c, strings.Contains(out, "rename"), "Missing 'rename' log event")
}

func (s *DockerCLIEventSuite) TestEventsTop(c *testing.T) {
	// Problematic on Windows as Windows does not support top
	testRequires(c, DaemonIsLinux)

	cID := runSleepingContainer(c, "-d")
	cli.WaitRun(c, cID)

	cli.DockerCmd(c, "top", cID)
	cli.DockerCmd(c, "stop", cID)

	until := daemonUnixTime(c)
	out := cli.DockerCmd(c, "events", "-f", "container="+cID, "--until="+until).Combined()
	assert.Assert(c, strings.Contains(out, "top"), "Missing 'top' log event")
}

// #14316
func (s *DockerRegistrySuite) TestEventsImageFilterPush(c *testing.T) {
	// Problematic to port for Windows CI during TP5 timeframe until
	// supporting push
	testRequires(c, DaemonIsLinux)
	testRequires(c, Network)
	imgRepoName := fmt.Sprintf("%v/dockercli/testf", privateRegistryURL)

	out := cli.DockerCmd(c, "run", "-d", "busybox", "top").Stdout()
	cID := strings.TrimSpace(out)
	cli.WaitRun(c, cID)

	cli.DockerCmd(c, "commit", cID, imgRepoName)
	cli.DockerCmd(c, "stop", cID)
	cli.DockerCmd(c, "push", imgRepoName)

	until := daemonUnixTime(c)
	out = cli.DockerCmd(c, "events", "-f", "image="+imgRepoName, "-f", "event=push", "--until", until).Stdout()
	assert.Assert(c, strings.Contains(out, imgRepoName), "Missing 'push' log event for %s", imgRepoName)
}

func (s *DockerCLIEventSuite) TestEventsFilterType(c *testing.T) {
	// FIXME(vdemeester) fails on e2e run
	testRequires(c, testEnv.IsLocalDaemon)
	since := daemonUnixTime(c)
	name := "labelfiltertest"
	label := "io.docker.testing=image"

	// Build a test image.
	buildImageSuccessfully(c, name, build.WithDockerfile(fmt.Sprintf(`
		FROM busybox:latest
		LABEL %s`, label)))
	cli.DockerCmd(c, "tag", name, "labelfiltertest:tag1")
	cli.DockerCmd(c, "tag", name, "labelfiltertest:tag2")
	cli.DockerCmd(c, "tag", "busybox:latest", "labelfiltertest:tag3")

	out := cli.DockerCmd(c,
		"events",
		"--since", since,
		"--until", daemonUnixTime(c),
		"--filter", fmt.Sprintf("label=%s", label),
		"--filter", "type=image",
	).Stdout()

	events := strings.Split(strings.TrimSpace(out), "\n")

	// 2 events from the "docker tag" command, another one is from "docker build"
	assert.Equal(c, len(events), 3, "Events == %s", events)
	for _, e := range events {
		assert.Check(c, strings.Contains(e, "labelfiltertest"))
	}

	out = cli.DockerCmd(c,
		"events",
		"--since", since,
		"--until", daemonUnixTime(c),
		"--filter", fmt.Sprintf("label=%s", label),
		"--filter", "type=container",
	).Stdout()
	events = strings.Split(strings.TrimSpace(out), "\n")

	// Events generated by the container that builds the image
	assert.Equal(c, len(events), 2, "Events == %s", events)

	out = cli.DockerCmd(c,
		"events",
		"--since", since,
		"--until", daemonUnixTime(c),
		"--filter", "type=network",
	).Stdout()
	events = strings.Split(strings.TrimSpace(out), "\n")
	assert.Assert(c, len(events) >= 1, "Events == %s", events)
}

// #25798
func (s *DockerCLIEventSuite) TestEventsSpecialFiltersWithExecCreate(c *testing.T) {
	since := daemonUnixTime(c)
	runSleepingContainer(c, "--name", "test-container", "-d")
	cli.WaitRun(c, "test-container")

	cli.DockerCmd(c, "exec", "test-container", "echo", "hello-world")

	out := cli.DockerCmd(c,
		"events",
		"--since", since,
		"--until", daemonUnixTime(c),
		"--filter",
		"event='exec_create: echo hello-world'",
	).Stdout()

	events := strings.Split(strings.TrimSpace(out), "\n")
	assert.Equal(c, len(events), 1, out)

	out = cli.DockerCmd(c,
		"events",
		"--since", since,
		"--until", daemonUnixTime(c),
		"--filter",
		"event=exec_create",
	).Stdout()
	assert.Equal(c, len(events), 1, out)
}

func (s *DockerCLIEventSuite) TestEventsFilterImageInContainerAction(c *testing.T) {
	since := daemonUnixTime(c)
	cli.DockerCmd(c, "run", "-d", "busybox", "true")

	out := cli.DockerCmd(c, "events", "--filter", "image=busybox", "--since", since, "--until", daemonUnixTime(c)).Stdout()
	events := strings.Split(strings.TrimSpace(out), "\n")
	assert.Assert(c, len(events) > 1, out)
}

func (s *DockerCLIEventSuite) TestEventsContainerRestart(c *testing.T) {
	cli.DockerCmd(c, "run", "-d", "--name=testEvent", "--restart=on-failure:3", "busybox", "false")

	// wait until test2 is auto removed.
	waitTime := 10 * time.Second
	if testEnv.DaemonInfo.OSType == "windows" {
		// Windows takes longer...
		waitTime = 90 * time.Second
	}

	err := waitInspect("testEvent", "{{ .State.Restarting }} {{ .State.Running }}", "false false", waitTime)
	assert.NilError(c, err)

	var (
		createCount int
		startCount  int
		dieCount    int
	)
	out := cli.DockerCmd(c, "events", "--since=0", "--until", daemonUnixTime(c), "-f", "container=testEvent").Stdout()
	events := strings.Split(strings.TrimSpace(out), "\n")

	nEvents := len(events)
	assert.Assert(c, nEvents >= 1)
	actions := eventActionsByIDAndType(c, events, "testEvent", "container")

	for _, a := range actions {
		switch a {
		case "create":
			createCount++
		case "start":
			startCount++
		case "die":
			dieCount++
		}
	}
	assert.Equal(c, createCount, 1, "testEvent should be created 1 times: %v", actions)
	assert.Equal(c, startCount, 4, "testEvent should start 4 times: %v", actions)
	assert.Equal(c, dieCount, 4, "testEvent should die 4 times: %v", actions)
}

func (s *DockerCLIEventSuite) TestEventsSinceInTheFuture(c *testing.T) {
	cli.DockerCmd(c, "run", "--name", "test-container", "-d", "busybox", "true")

	since := daemonTime(c)
	until := since.Add(time.Duration(-24) * time.Hour)
	out, _, err := dockerCmdWithError("events", "--filter", "image=busybox", "--since", parseEventTime(since), "--until", parseEventTime(until))

	assert.ErrorContains(c, err, "")
	assert.Assert(c, strings.Contains(out, "cannot be after `until`"))
}

func (s *DockerCLIEventSuite) TestEventsUntilInThePast(c *testing.T) {
	since := daemonUnixTime(c)

	cli.DockerCmd(c, "run", "--name", "test-container", "-d", "busybox", "true")

	until := daemonUnixTime(c)

	cli.DockerCmd(c, "run", "--name", "test-container2", "-d", "busybox", "true")

	out := cli.DockerCmd(c, "events", "--filter", "image=busybox", "--since", since, "--until", until).Stdout()

	assert.Assert(c, !strings.Contains(out, "test-container2"))
	assert.Assert(c, strings.Contains(out, "test-container"))
}

func (s *DockerCLIEventSuite) TestEventsFormat(c *testing.T) {
	since := daemonUnixTime(c)
	cli.DockerCmd(c, "run", "--rm", "busybox", "true")
	cli.DockerCmd(c, "run", "--rm", "busybox", "true")
	out := cli.DockerCmd(c, "events", "--since", since, "--until", daemonUnixTime(c), "--format", "{{json .}}").Stdout()
	dec := json.NewDecoder(strings.NewReader(out))
	// make sure we got 2 start events
	startCount := 0
	for {
		var err error
		var ev eventtypes.Message
		if err = dec.Decode(&ev); err == io.EOF {
			break
		}
		assert.NilError(c, err)
		if ev.Action == eventtypes.ActionStart {
			startCount++
		}
	}

	assert.Equal(c, startCount, 2, "should have had 2 start events but had %d, out: %s", startCount, out)
}

func (s *DockerCLIEventSuite) TestEventsFormatBadFunc(c *testing.T) {
	// make sure it fails immediately, without receiving any event
	result := cli.Docker(cli.Args("events", "--format", "{{badFuncString .}}"))
	result.Assert(c, icmd.Expected{
		Error:    "exit status 64",
		ExitCode: 64,
		Err:      `Error parsing format: template: :1: function "badFuncString" not defined`,
	})
}

func (s *DockerCLIEventSuite) TestEventsFormatBadField(c *testing.T) {
	// make sure it fails immediately, without receiving any event
	result := cli.Docker(cli.Args("events", "--format", "{{.badFieldString}}"))
	result.Assert(c, icmd.Expected{
		Error:    "exit status 64",
		ExitCode: 64,
		Err:      `Error parsing format: template: :1:2: executing "" at <.badFieldString>: can't evaluate field badFieldString in type *events.Message`,
	})
}
