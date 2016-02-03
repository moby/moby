package main

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/pkg/integration/checker"
	"github.com/go-check/check"
)

func (s *DockerSuite) TestEventsTimestampFormats(c *check.C) {
	image := "busybox"

	// Start stopwatch, generate an event
	time.Sleep(1 * time.Second) // so that we don't grab events from previous test occured in the same second
	start := daemonTime(c)
	dockerCmd(c, "tag", image, "timestamptest:1")
	dockerCmd(c, "rmi", "timestamptest:1")
	time.Sleep(1 * time.Second) // so that until > since
	end := daemonTime(c)

	// List of available time formats to --since
	unixTs := func(t time.Time) string { return fmt.Sprintf("%v", t.Unix()) }
	rfc3339 := func(t time.Time) string { return t.Format(time.RFC3339) }
	duration := func(t time.Time) string { return time.Now().Sub(t).String() }

	// --since=$start must contain only the 'untag' event
	for _, f := range []func(time.Time) string{unixTs, rfc3339, duration} {
		since, until := f(start), f(end)
		out, _ := dockerCmd(c, "events", "--since="+since, "--until="+until)
		events := strings.Split(strings.TrimSpace(out), "\n")
		c.Assert(events, checker.HasLen, 2, check.Commentf("unexpected events, was expecting only 2 events tag/untag (since=%s, until=%s) out=%s", since, until, out))
		c.Assert(out, checker.Contains, "untag", check.Commentf("expected 'untag' event not found (since=%s, until=%s)", since, until))
	}

}

func (s *DockerSuite) TestEventsUntag(c *check.C) {
	image := "busybox"
	dockerCmd(c, "tag", image, "utest:tag1")
	dockerCmd(c, "tag", image, "utest:tag2")
	dockerCmd(c, "rmi", "utest:tag1")
	dockerCmd(c, "rmi", "utest:tag2")
	eventsCmd := exec.Command(dockerBinary, "events", "--since=1")
	out, exitCode, _, err := runCommandWithOutputForDuration(eventsCmd, time.Duration(time.Millisecond*2500))
	c.Assert(err, checker.IsNil)
	c.Assert(exitCode, checker.Equals, 0, check.Commentf("Failed to get events"))
	events := strings.Split(out, "\n")
	nEvents := len(events)
	// The last element after the split above will be an empty string, so we
	// get the two elements before the last, which are the untags we're
	// looking for.
	for _, v := range events[nEvents-3 : nEvents-1] {
		c.Assert(v, checker.Contains, "untag", check.Commentf("event should be untag"))
	}
}

func (s *DockerSuite) TestEventsContainerFailStartDie(c *check.C) {
	_, _, err := dockerCmdWithError("run", "--name", "testeventdie", "busybox", "blerg")
	c.Assert(err, checker.NotNil, check.Commentf("Container run with command blerg should have failed, but it did not"))

	out, _ := dockerCmd(c, "events", "--since=0", fmt.Sprintf("--until=%d", daemonTime(c).Unix()))
	events := strings.Split(strings.TrimSpace(out), "\n")

	nEvents := len(events)
	c.Assert(nEvents, checker.GreaterOrEqualThan, 1) //Missing expected event

	actions := eventActionsByIDAndType(c, events, "testeventdie", "container")

	var startEvent bool
	var dieEvent bool
	for _, a := range actions {
		switch a {
		case "start":
			startEvent = true
		case "die":
			dieEvent = true
		}
	}
	c.Assert(startEvent, checker.True, check.Commentf("Start event not found: %v\n%v", actions, events))
	c.Assert(dieEvent, checker.True, check.Commentf("Die event not found: %v\n%v", actions, events))
}

func (s *DockerSuite) TestEventsLimit(c *check.C) {
	// TODO Windows CI: This test is not reliable enough on Windows TP4. Reports
	// multiple errors in the analytic log sometimes.
	// [NetSetupHelper::InstallVirtualMiniport()@2153] NetSetup install of ROOT\VMS_MP\0001 failed with error 0x80070002
	// This should be able to be enabled on TP5.
	testRequires(c, DaemonIsLinux)
	var waitGroup sync.WaitGroup
	errChan := make(chan error, 17)

	args := []string{"run", "--rm", "busybox", "true"}
	for i := 0; i < 17; i++ {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			errChan <- exec.Command(dockerBinary, args...).Run()
		}()
	}

	waitGroup.Wait()
	close(errChan)

	for err := range errChan {
		c.Assert(err, checker.IsNil, check.Commentf("%q failed with error", strings.Join(args, " ")))
	}

	out, _ := dockerCmd(c, "events", "--since=0", fmt.Sprintf("--until=%d", daemonTime(c).Unix()))
	events := strings.Split(out, "\n")
	nEvents := len(events) - 1
	c.Assert(nEvents, checker.Equals, 64, check.Commentf("events should be limited to 64, but received %d", nEvents))
}

func (s *DockerSuite) TestEventsContainerEvents(c *check.C) {
	containerID, _ := dockerCmd(c, "run", "--rm", "--name", "container-events-test", "busybox", "true")
	containerID = strings.TrimSpace(containerID)

	out, _ := dockerCmd(c, "events", "--since=0", fmt.Sprintf("--until=%d", daemonTime(c).Unix()))
	events := strings.Split(out, "\n")
	events = events[:len(events)-1]

	nEvents := len(events)
	c.Assert(nEvents, checker.GreaterOrEqualThan, 5) //Missing expected event
	containerEvents := eventActionsByIDAndType(c, events, "container-events-test", "container")
	c.Assert(containerEvents, checker.HasLen, 5, check.Commentf("events: %v", events))

	c.Assert(containerEvents[0], checker.Equals, "create", check.Commentf(out))
	c.Assert(containerEvents[1], checker.Equals, "attach", check.Commentf(out))
	c.Assert(containerEvents[2], checker.Equals, "start", check.Commentf(out))
	c.Assert(containerEvents[3], checker.Equals, "die", check.Commentf(out))
	c.Assert(containerEvents[4], checker.Equals, "destroy", check.Commentf(out))
}

func (s *DockerSuite) TestEventsContainerEventsAttrSort(c *check.C) {
	since := daemonTime(c).Unix()
	containerID, _ := dockerCmd(c, "run", "-d", "--name", "container-events-test", "busybox", "true")
	containerID = strings.TrimSpace(containerID)

	out, _ := dockerCmd(c, "events", fmt.Sprintf("--since=%d", since), fmt.Sprintf("--until=%d", daemonTime(c).Unix()))
	events := strings.Split(out, "\n")

	nEvents := len(events)
	c.Assert(nEvents, checker.GreaterOrEqualThan, 3) //Missing expected event
	matchedEvents := 0
	for _, event := range events {
		matches := parseEventText(event)
		if matches["id"] != containerID {
			continue
		}
		if matches["eventType"] == "container" && matches["action"] == "create" {
			matchedEvents++
			c.Assert(out, checker.Contains, "(image=busybox, name=container-events-test)", check.Commentf("Event attributes not sorted"))
		} else if matches["eventType"] == "container" && matches["action"] == "start" {
			matchedEvents++
			c.Assert(out, checker.Contains, "(image=busybox, name=container-events-test)", check.Commentf("Event attributes not sorted"))
		}
	}
	c.Assert(matchedEvents, checker.Equals, 2)
}

func (s *DockerSuite) TestEventsContainerEventsSinceUnixEpoch(c *check.C) {
	dockerCmd(c, "run", "--rm", "--name", "since-epoch-test", "busybox", "true")
	timeBeginning := time.Unix(0, 0).Format(time.RFC3339Nano)
	timeBeginning = strings.Replace(timeBeginning, "Z", ".000000000Z", -1)
	out, _ := dockerCmd(c, "events", fmt.Sprintf("--since='%s'", timeBeginning), fmt.Sprintf("--until=%d", daemonTime(c).Unix()))
	events := strings.Split(out, "\n")
	events = events[:len(events)-1]

	nEvents := len(events)
	c.Assert(nEvents, checker.GreaterOrEqualThan, 5) //Missing expected event
	containerEvents := eventActionsByIDAndType(c, events, "since-epoch-test", "container")
	c.Assert(containerEvents, checker.HasLen, 5, check.Commentf("events: %v", events))

	c.Assert(containerEvents[0], checker.Equals, "create", check.Commentf(out))
	c.Assert(containerEvents[1], checker.Equals, "attach", check.Commentf(out))
	c.Assert(containerEvents[2], checker.Equals, "start", check.Commentf(out))
	c.Assert(containerEvents[3], checker.Equals, "die", check.Commentf(out))
	c.Assert(containerEvents[4], checker.Equals, "destroy", check.Commentf(out))
}

func (s *DockerSuite) TestEventsImageTag(c *check.C) {
	time.Sleep(1 * time.Second) // because API has seconds granularity
	since := daemonTime(c).Unix()
	image := "testimageevents:tag"
	dockerCmd(c, "tag", "busybox", image)

	out, _ := dockerCmd(c, "events",
		fmt.Sprintf("--since=%d", since),
		fmt.Sprintf("--until=%d", daemonTime(c).Unix()))

	events := strings.Split(strings.TrimSpace(out), "\n")
	c.Assert(events, checker.HasLen, 1, check.Commentf("was expecting 1 event. out=%s", out))
	event := strings.TrimSpace(events[0])

	matches := parseEventText(event)
	c.Assert(matchEventID(matches, image), checker.True, check.Commentf("matches: %v\nout:\n%s", matches, out))
	c.Assert(matches["action"], checker.Equals, "tag")
}

func (s *DockerSuite) TestEventsImagePull(c *check.C) {
	// TODO Windows: Enable this test once pull and reliable image names are available
	testRequires(c, DaemonIsLinux)
	since := daemonTime(c).Unix()
	testRequires(c, Network)

	dockerCmd(c, "pull", "hello-world")

	out, _ := dockerCmd(c, "events",
		fmt.Sprintf("--since=%d", since),
		fmt.Sprintf("--until=%d", daemonTime(c).Unix()))

	events := strings.Split(strings.TrimSpace(out), "\n")
	event := strings.TrimSpace(events[len(events)-1])
	matches := parseEventText(event)
	c.Assert(matches["id"], checker.Equals, "hello-world:latest")
	c.Assert(matches["action"], checker.Equals, "pull")

}

func (s *DockerSuite) TestEventsImageImport(c *check.C) {
	// TODO Windows CI. This should be portable once export/import are
	// more reliable (@swernli)
	testRequires(c, DaemonIsLinux)

	out, _ := dockerCmd(c, "run", "-d", "busybox", "true")
	cleanedContainerID := strings.TrimSpace(out)

	since := daemonTime(c).Unix()
	out, _, err := runCommandPipelineWithOutput(
		exec.Command(dockerBinary, "export", cleanedContainerID),
		exec.Command(dockerBinary, "import", "-"),
	)
	c.Assert(err, checker.IsNil, check.Commentf("import failed with output: %q", out))
	imageRef := strings.TrimSpace(out)

	out, _ = dockerCmd(c, "events", fmt.Sprintf("--since=%d", since), fmt.Sprintf("--until=%d", daemonTime(c).Unix()), "--filter", "event=import")
	events := strings.Split(strings.TrimSpace(out), "\n")
	c.Assert(events, checker.HasLen, 1)
	matches := parseEventText(events[0])
	c.Assert(matches["id"], checker.Equals, imageRef, check.Commentf("matches: %v\nout:\n%s\n", matches, out))
	c.Assert(matches["action"], checker.Equals, "import", check.Commentf("matches: %v\nout:\n%s\n", matches, out))
}

func (s *DockerSuite) TestEventsFilters(c *check.C) {
	since := daemonTime(c).Unix()
	dockerCmd(c, "run", "--rm", "busybox", "true")
	dockerCmd(c, "run", "--rm", "busybox", "true")
	out, _ := dockerCmd(c, "events", fmt.Sprintf("--since=%d", since), fmt.Sprintf("--until=%d", daemonTime(c).Unix()), "--filter", "event=die")
	parseEvents(c, out, "die")

	out, _ = dockerCmd(c, "events", fmt.Sprintf("--since=%d", since), fmt.Sprintf("--until=%d", daemonTime(c).Unix()), "--filter", "event=die", "--filter", "event=start")
	parseEvents(c, out, "die|start")

	// make sure we at least got 2 start events
	count := strings.Count(out, "start")
	c.Assert(strings.Count(out, "start"), checker.GreaterOrEqualThan, 2, check.Commentf("should have had 2 start events but had %d, out: %s", count, out))

}

func (s *DockerSuite) TestEventsFilterImageName(c *check.C) {
	since := daemonTime(c).Unix()

	out, _ := dockerCmd(c, "run", "--name", "container_1", "-d", "busybox:latest", "true")
	container1 := strings.TrimSpace(out)

	out, _ = dockerCmd(c, "run", "--name", "container_2", "-d", "busybox", "true")
	container2 := strings.TrimSpace(out)

	name := "busybox"
	out, _ = dockerCmd(c, "events", fmt.Sprintf("--since=%d", since), fmt.Sprintf("--until=%d", daemonTime(c).Unix()), "--filter", fmt.Sprintf("image=%s", name))
	events := strings.Split(out, "\n")
	events = events[:len(events)-1]
	c.Assert(events, checker.Not(checker.HasLen), 0) //Expected events but found none for the image busybox:latest
	count1 := 0
	count2 := 0

	for _, e := range events {
		if strings.Contains(e, container1) {
			count1++
		} else if strings.Contains(e, container2) {
			count2++
		}
	}
	c.Assert(count1, checker.Not(checker.Equals), 0, check.Commentf("Expected event from container but got %d from %s", count1, container1))
	c.Assert(count2, checker.Not(checker.Equals), 0, check.Commentf("Expected event from container but got %d from %s", count2, container2))

}

func (s *DockerSuite) TestEventsFilterLabels(c *check.C) {
	since := daemonTime(c).Unix()
	label := "io.docker.testing=foo"

	out, _ := dockerCmd(c, "run", "-d", "-l", label, "busybox:latest", "true")
	container1 := strings.TrimSpace(out)

	out, _ = dockerCmd(c, "run", "-d", "busybox", "true")
	container2 := strings.TrimSpace(out)

	out, _ = dockerCmd(
		c,
		"events",
		fmt.Sprintf("--since=%d", since),
		fmt.Sprintf("--until=%d", daemonTime(c).Unix()),
		"--filter", fmt.Sprintf("label=%s", label))

	events := strings.Split(strings.TrimSpace(out), "\n")
	c.Assert(len(events), checker.Equals, 3)

	for _, e := range events {
		c.Assert(e, checker.Contains, container1)
		c.Assert(e, checker.Not(checker.Contains), container2)
	}
}

func (s *DockerSuite) TestEventsFilterImageLabels(c *check.C) {
	since := daemonTime(c).Unix()
	name := "labelfiltertest"
	label := "io.docker.testing=image"

	// Build a test image.
	_, err := buildImage(name, fmt.Sprintf(`
		FROM busybox:latest
		LABEL %s`, label), true)
	c.Assert(err, checker.IsNil, check.Commentf("Couldn't create image"))

	dockerCmd(c, "tag", name, "labelfiltertest:tag1")
	dockerCmd(c, "tag", name, "labelfiltertest:tag2")
	dockerCmd(c, "tag", "busybox:latest", "labelfiltertest:tag3")

	out, _ := dockerCmd(
		c,
		"events",
		fmt.Sprintf("--since=%d", since),
		fmt.Sprintf("--until=%d", daemonTime(c).Unix()),
		"--filter", fmt.Sprintf("label=%s", label),
		"--filter", "type=image")

	events := strings.Split(strings.TrimSpace(out), "\n")

	// 2 events from the "docker tag" command, another one is from "docker build"
	c.Assert(events, checker.HasLen, 3, check.Commentf("Events == %s", events))
	for _, e := range events {
		c.Assert(e, checker.Contains, "labelfiltertest")
	}
}

func (s *DockerSuite) TestEventsFilterContainer(c *check.C) {
	since := fmt.Sprintf("%d", daemonTime(c).Unix())
	nameID := make(map[string]string)

	for _, name := range []string{"container_1", "container_2"} {
		dockerCmd(c, "run", "--name", name, "busybox", "true")
		id := inspectField(c, name, "Id")
		nameID[name] = id
	}

	until := fmt.Sprintf("%d", daemonTime(c).Unix())

	checkEvents := func(id string, events []string) error {
		if len(events) != 4 { // create, attach, start, die
			return fmt.Errorf("expected 4 events, got %v", events)
		}
		for _, event := range events {
			matches := parseEventText(event)
			if !matchEventID(matches, id) {
				return fmt.Errorf("expected event for container id %s: %s - parsed container id: %s", id, event, matches["id"])
			}
		}
		return nil
	}

	for name, ID := range nameID {
		// filter by names
		out, _ := dockerCmd(c, "events", "--since", since, "--until", until, "--filter", "container="+name)
		events := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
		c.Assert(checkEvents(ID, events), checker.IsNil)

		// filter by ID's
		out, _ = dockerCmd(c, "events", "--since", since, "--until", until, "--filter", "container="+ID)
		events = strings.Split(strings.TrimSuffix(out, "\n"), "\n")
		c.Assert(checkEvents(ID, events), checker.IsNil)
	}
}

func (s *DockerSuite) TestEventsCommit(c *check.C) {
	// Problematic on Windows as cannot commit a running container
	testRequires(c, DaemonIsLinux)
	since := daemonTime(c).Unix()

	out, _ := runSleepingContainer(c, "-d")
	cID := strings.TrimSpace(out)
	c.Assert(waitRun(cID), checker.IsNil)

	dockerCmd(c, "commit", "-m", "test", cID)
	dockerCmd(c, "stop", cID)

	out, _ = dockerCmd(c, "events", "--since=0", "-f", "container="+cID, "--until="+strconv.Itoa(int(since)))
	c.Assert(out, checker.Contains, "commit", check.Commentf("Missing 'commit' log event"))
}

func (s *DockerSuite) TestEventsCopy(c *check.C) {
	since := daemonTime(c).Unix()

	// Build a test image.
	id, err := buildImage("cpimg", `
		  FROM busybox
		  RUN echo HI > /file`, true)
	c.Assert(err, checker.IsNil, check.Commentf("Couldn't create image"))

	// Create an empty test file.
	tempFile, err := ioutil.TempFile("", "test-events-copy-")
	c.Assert(err, checker.IsNil)
	defer os.Remove(tempFile.Name())

	c.Assert(tempFile.Close(), checker.IsNil)

	dockerCmd(c, "create", "--name=cptest", id)

	dockerCmd(c, "cp", "cptest:/file", tempFile.Name())

	out, _ := dockerCmd(c, "events", "--since=0", "-f", "container=cptest", "--until="+strconv.Itoa(int(since)))
	c.Assert(out, checker.Contains, "archive-path", check.Commentf("Missing 'archive-path' log event\n"))

	dockerCmd(c, "cp", tempFile.Name(), "cptest:/filecopy")

	out, _ = dockerCmd(c, "events", "--since=0", "-f", "container=cptest", "--until="+strconv.Itoa(int(since)))
	c.Assert(out, checker.Contains, "extract-to-dir", check.Commentf("Missing 'extract-to-dir' log event"))
}

func (s *DockerSuite) TestEventsResize(c *check.C) {
	since := daemonTime(c).Unix()

	out, _ := dockerCmd(c, "run", "-d", "busybox", "top")
	cID := strings.TrimSpace(out)
	c.Assert(waitRun(cID), checker.IsNil)

	endpoint := "/containers/" + cID + "/resize?h=80&w=24"
	status, _, err := sockRequest("POST", endpoint, nil)
	c.Assert(status, checker.Equals, http.StatusOK)
	c.Assert(err, checker.IsNil)

	dockerCmd(c, "stop", cID)

	out, _ = dockerCmd(c, "events", "--since=0", "-f", "container="+cID, "--until="+strconv.Itoa(int(since)))
	c.Assert(out, checker.Contains, "resize", check.Commentf("Missing 'resize' log event"))
}

func (s *DockerSuite) TestEventsAttach(c *check.C) {
	// TODO Windows CI: Figure out why this test fails intermittently (TP4 and TP5).
	testRequires(c, DaemonIsLinux)
	since := daemonTime(c).Unix()

	out, _ := dockerCmd(c, "run", "-di", "busybox", "cat")
	cID := strings.TrimSpace(out)

	cmd := exec.Command(dockerBinary, "attach", cID)
	stdin, err := cmd.StdinPipe()
	c.Assert(err, checker.IsNil)
	defer stdin.Close()
	stdout, err := cmd.StdoutPipe()
	c.Assert(err, checker.IsNil)
	defer stdout.Close()
	c.Assert(cmd.Start(), checker.IsNil)
	defer cmd.Process.Kill()

	// Make sure we're done attaching by writing/reading some stuff
	_, err = stdin.Write([]byte("hello\n"))
	c.Assert(err, checker.IsNil)
	out, err = bufio.NewReader(stdout).ReadString('\n')
	c.Assert(err, checker.IsNil)
	c.Assert(strings.TrimSpace(out), checker.Equals, "hello", check.Commentf("expected 'hello'"))

	c.Assert(stdin.Close(), checker.IsNil)

	dockerCmd(c, "kill", cID)

	out, _ = dockerCmd(c, "events", "--since=0", "-f", "container="+cID, "--until="+strconv.Itoa(int(since)))
	c.Assert(out, checker.Contains, "attach", check.Commentf("Missing 'attach' log event"))
}

func (s *DockerSuite) TestEventsRename(c *check.C) {
	since := daemonTime(c).Unix()

	dockerCmd(c, "run", "--name", "oldName", "busybox", "true")
	dockerCmd(c, "rename", "oldName", "newName")

	out, _ := dockerCmd(c, "events", "--since=0", "-f", "container=newName", "--until="+strconv.Itoa(int(since)))
	c.Assert(out, checker.Contains, "rename", check.Commentf("Missing 'rename' log event\n"))
}

func (s *DockerSuite) TestEventsTop(c *check.C) {
	// Problematic on Windows as Windows does not support top
	testRequires(c, DaemonIsLinux)
	since := daemonTime(c).Unix()

	out, _ := runSleepingContainer(c, "-d")
	cID := strings.TrimSpace(out)
	c.Assert(waitRun(cID), checker.IsNil)

	dockerCmd(c, "top", cID)
	dockerCmd(c, "stop", cID)

	out, _ = dockerCmd(c, "events", "--since=0", "-f", "container="+cID, "--until="+strconv.Itoa(int(since)))
	c.Assert(out, checker.Contains, " top", check.Commentf("Missing 'top' log event"))
}

// #13753
func (s *DockerSuite) TestEventsDefaultEmpty(c *check.C) {
	dockerCmd(c, "run", "busybox")
	out, _ := dockerCmd(c, "events", fmt.Sprintf("--until=%d", daemonTime(c).Unix()))
	c.Assert(strings.TrimSpace(out), checker.Equals, "")
}

// #14316
func (s *DockerRegistrySuite) TestEventsImageFilterPush(c *check.C) {
	// Problematic to port for Windows CI during TP4/TP5 timeframe while
	// not supporting push
	testRequires(c, DaemonIsLinux)
	testRequires(c, Network)
	since := daemonTime(c).Unix()
	repoName := fmt.Sprintf("%v/dockercli/testf", privateRegistryURL)

	out, _ := dockerCmd(c, "run", "-d", "busybox", "top")
	cID := strings.TrimSpace(out)
	c.Assert(waitRun(cID), checker.IsNil)

	dockerCmd(c, "commit", cID, repoName)
	dockerCmd(c, "stop", cID)
	dockerCmd(c, "push", repoName)

	out, _ = dockerCmd(c, "events", "--since=0", "-f", "image="+repoName, "-f", "event=push", "--until="+strconv.Itoa(int(since)))
	c.Assert(out, checker.Contains, repoName, check.Commentf("Missing 'push' log event for %s", repoName))
}

func (s *DockerSuite) TestEventsFilterType(c *check.C) {
	since := daemonTime(c).Unix()
	name := "labelfiltertest"
	label := "io.docker.testing=image"

	// Build a test image.
	_, err := buildImage(name, fmt.Sprintf(`
		FROM busybox:latest
		LABEL %s`, label), true)
	c.Assert(err, checker.IsNil, check.Commentf("Couldn't create image"))

	dockerCmd(c, "tag", name, "labelfiltertest:tag1")
	dockerCmd(c, "tag", name, "labelfiltertest:tag2")
	dockerCmd(c, "tag", "busybox:latest", "labelfiltertest:tag3")

	out, _ := dockerCmd(
		c,
		"events",
		fmt.Sprintf("--since=%d", since),
		fmt.Sprintf("--until=%d", daemonTime(c).Unix()),
		"--filter", fmt.Sprintf("label=%s", label),
		"--filter", "type=image")

	events := strings.Split(strings.TrimSpace(out), "\n")

	// 2 events from the "docker tag" command, another one is from "docker build"
	c.Assert(events, checker.HasLen, 3, check.Commentf("Events == %s", events))
	for _, e := range events {
		c.Assert(e, checker.Contains, "labelfiltertest")
	}

	out, _ = dockerCmd(
		c,
		"events",
		fmt.Sprintf("--since=%d", since),
		fmt.Sprintf("--until=%d", daemonTime(c).Unix()),
		"--filter", fmt.Sprintf("label=%s", label),
		"--filter", "type=container")
	events = strings.Split(strings.TrimSpace(out), "\n")

	// Events generated by the container that builds the image
	c.Assert(events, checker.HasLen, 3, check.Commentf("Events == %s", events))

	out, _ = dockerCmd(
		c,
		"events",
		fmt.Sprintf("--since=%d", since),
		fmt.Sprintf("--until=%d", daemonTime(c).Unix()),
		"--filter", "type=network")
	events = strings.Split(strings.TrimSpace(out), "\n")
	c.Assert(len(events), checker.GreaterOrEqualThan, 1, check.Commentf("Events == %s", events))
}

func (s *DockerSuite) TestEventsFilterImageInContainerAction(c *check.C) {
	since := daemonTime(c).Unix()
	dockerCmd(c, "run", "--name", "test-container", "-d", "busybox", "true")
	waitRun("test-container")

	out, _ := dockerCmd(c, "events", "--filter", "image=busybox", fmt.Sprintf("--since=%d", since), fmt.Sprintf("--until=%d", daemonTime(c).Unix()))
	events := strings.Split(strings.TrimSpace(out), "\n")
	c.Assert(len(events), checker.GreaterThan, 1, check.Commentf(out))
}
