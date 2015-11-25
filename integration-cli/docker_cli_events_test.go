package main

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/pkg/integration/checker"
	"github.com/go-check/check"
)

func (s *DockerSuite) TestEventsTimestampFormats(c *check.C) {
	testRequires(c, DaemonIsLinux)
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
	testRequires(c, DaemonIsLinux)
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

	out, _ := dockerCmd(c, "images", "-q")
	image := strings.Split(out, "\n")[0]
	_, _, err := dockerCmdWithError("run", "--name", "testeventdie", image, "blerg")
	c.Assert(err, checker.NotNil, check.Commentf("Container run with command blerg should have failed, but it did not, out=%s", out))

	out, _ = dockerCmd(c, "events", "--since=0", fmt.Sprintf("--until=%d", daemonTime(c).Unix()))
	events := strings.Split(out, "\n")
	c.Assert(len(events), checker.GreaterThan, 1) //Missing expected event

	startEvent := strings.Fields(events[len(events)-3])
	dieEvent := strings.Fields(events[len(events)-2])

	c.Assert(startEvent[len(startEvent)-1], checker.Equals, "start", check.Commentf("event should be start, not %#v", startEvent))
	c.Assert(dieEvent[len(dieEvent)-1], checker.Equals, "die", check.Commentf("event should be die, not %#v", dieEvent))

}

func (s *DockerSuite) TestEventsLimit(c *check.C) {
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
	testRequires(c, DaemonIsLinux)
	dockerCmd(c, "run", "--rm", "busybox", "true")
	out, _ := dockerCmd(c, "events", "--since=0", fmt.Sprintf("--until=%d", daemonTime(c).Unix()))
	events := strings.Split(out, "\n")
	events = events[:len(events)-1]
	c.Assert(len(events), checker.GreaterOrEqualThan, 5) //Missing expected event
	createEvent := strings.Fields(events[len(events)-5])
	attachEvent := strings.Fields(events[len(events)-4])
	startEvent := strings.Fields(events[len(events)-3])
	dieEvent := strings.Fields(events[len(events)-2])
	destroyEvent := strings.Fields(events[len(events)-1])
	c.Assert(createEvent[len(createEvent)-1], checker.Equals, "create", check.Commentf("event should be create, not %#v", createEvent))
	c.Assert(attachEvent[len(attachEvent)-1], checker.Equals, "attach", check.Commentf("event should be attach, not %#v", attachEvent))
	c.Assert(startEvent[len(startEvent)-1], checker.Equals, "start", check.Commentf("event should be start, not %#v", startEvent))
	c.Assert(dieEvent[len(dieEvent)-1], checker.Equals, "die", check.Commentf("event should be die, not %#v", dieEvent))
	c.Assert(destroyEvent[len(destroyEvent)-1], checker.Equals, "destroy", check.Commentf("event should be destroy, not %#v", destroyEvent))

}

func (s *DockerSuite) TestEventsContainerEventsSinceUnixEpoch(c *check.C) {
	testRequires(c, DaemonIsLinux)
	dockerCmd(c, "run", "--rm", "busybox", "true")
	timeBeginning := time.Unix(0, 0).Format(time.RFC3339Nano)
	timeBeginning = strings.Replace(timeBeginning, "Z", ".000000000Z", -1)
	out, _ := dockerCmd(c, "events", fmt.Sprintf("--since='%s'", timeBeginning),
		fmt.Sprintf("--until=%d", daemonTime(c).Unix()))
	events := strings.Split(out, "\n")
	events = events[:len(events)-1]
	c.Assert(len(events), checker.GreaterOrEqualThan, 5) //Missing expected event
	createEvent := strings.Fields(events[len(events)-5])
	attachEvent := strings.Fields(events[len(events)-4])
	startEvent := strings.Fields(events[len(events)-3])
	dieEvent := strings.Fields(events[len(events)-2])
	destroyEvent := strings.Fields(events[len(events)-1])
	c.Assert(createEvent[len(createEvent)-1], checker.Equals, "create", check.Commentf("event should be create, not %#v", createEvent))
	c.Assert(attachEvent[len(attachEvent)-1], checker.Equals, "attach", check.Commentf("event should be attach, not %#v", attachEvent))
	c.Assert(startEvent[len(startEvent)-1], checker.Equals, "start", check.Commentf("event should be start, not %#v", startEvent))
	c.Assert(dieEvent[len(dieEvent)-1], checker.Equals, "die", check.Commentf("event should be die, not %#v", dieEvent))
	c.Assert(destroyEvent[len(destroyEvent)-1], checker.Equals, "destroy", check.Commentf("event should be destroy, not %#v", destroyEvent))

}

func (s *DockerSuite) TestEventsImageUntagDelete(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "testimageevents"
	_, err := buildImage(name,
		`FROM scratch
		MAINTAINER "docker"`,
		true)
	c.Assert(err, checker.IsNil)
	c.Assert(deleteImages(name), checker.IsNil)
	out, _ := dockerCmd(c, "events", "--since=0", fmt.Sprintf("--until=%d", daemonTime(c).Unix()))
	events := strings.Split(out, "\n")

	events = events[:len(events)-1]
	c.Assert(len(events), checker.GreaterOrEqualThan, 2) //Missing expected event
	untagEvent := strings.Fields(events[len(events)-2])
	deleteEvent := strings.Fields(events[len(events)-1])
	c.Assert(untagEvent[len(untagEvent)-1], checker.Equals, "untag", check.Commentf("untag should be untag, not %#v", untagEvent))
	c.Assert(deleteEvent[len(deleteEvent)-1], checker.Equals, "delete", check.Commentf("untag should be delete, not %#v", untagEvent))
}

func (s *DockerSuite) TestEventsImageTag(c *check.C) {
	testRequires(c, DaemonIsLinux)
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
	expectedStr := image + ": tag"

	c.Assert(event, checker.HasSuffix, expectedStr, check.Commentf("wrong event format. expected='%s' got=%s", expectedStr, event))

}

func (s *DockerSuite) TestEventsImagePull(c *check.C) {
	testRequires(c, DaemonIsLinux)
	since := daemonTime(c).Unix()
	testRequires(c, Network)

	dockerCmd(c, "pull", "hello-world")

	out, _ := dockerCmd(c, "events",
		fmt.Sprintf("--since=%d", since),
		fmt.Sprintf("--until=%d", daemonTime(c).Unix()))

	events := strings.Split(strings.TrimSpace(out), "\n")
	event := strings.TrimSpace(events[len(events)-1])

	c.Assert(event, checker.HasSuffix, "hello-world:latest: pull", check.Commentf("Missing pull event - got:%q", event))

}

func (s *DockerSuite) TestEventsImageImport(c *check.C) {
	testRequires(c, DaemonIsLinux)
	since := daemonTime(c).Unix()

	id := make(chan string)
	eventImport := make(chan struct{})
	eventsCmd := exec.Command(dockerBinary, "events", "--since", strconv.FormatInt(since, 10))
	stdout, err := eventsCmd.StdoutPipe()
	c.Assert(err, checker.IsNil)
	c.Assert(eventsCmd.Start(), checker.IsNil)
	defer eventsCmd.Process.Kill()

	go func() {
		containerID := <-id

		matchImport := regexp.MustCompile(containerID + `: import$`)
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			if matchImport.MatchString(scanner.Text()) {
				close(eventImport)
			}
		}
	}()

	out, _ := dockerCmd(c, "run", "-d", "busybox", "true")
	cleanedContainerID := strings.TrimSpace(out)

	out, _, err = runCommandPipelineWithOutput(
		exec.Command(dockerBinary, "export", cleanedContainerID),
		exec.Command(dockerBinary, "import", "-"),
	)
	c.Assert(err, checker.IsNil, check.Commentf("import failed with output: %q", out))
	newContainerID := strings.TrimSpace(out)
	id <- newContainerID

	select {
	case <-time.After(5 * time.Second):
		c.Fatal("failed to observe image import in timely fashion")
	case <-eventImport:
		// ignore, done
	}
}

func (s *DockerSuite) TestEventsFilters(c *check.C) {
	testRequires(c, DaemonIsLinux)
	parseEvents := func(out, match string) {
		events := strings.Split(out, "\n")
		events = events[:len(events)-1]
		for _, event := range events {
			eventFields := strings.Fields(event)
			eventName := eventFields[len(eventFields)-1]
			c.Assert(eventName, checker.Matches, match)
		}
	}

	since := daemonTime(c).Unix()
	dockerCmd(c, "run", "--rm", "busybox", "true")
	dockerCmd(c, "run", "--rm", "busybox", "true")
	out, _ := dockerCmd(c, "events", fmt.Sprintf("--since=%d", since), fmt.Sprintf("--until=%d", daemonTime(c).Unix()), "--filter", "event=die")
	parseEvents(out, "die")

	out, _ = dockerCmd(c, "events", fmt.Sprintf("--since=%d", since), fmt.Sprintf("--until=%d", daemonTime(c).Unix()), "--filter", "event=die", "--filter", "event=start")
	parseEvents(out, "((die)|(start))")

	// make sure we at least got 2 start events
	count := strings.Count(out, "start")
	c.Assert(strings.Count(out, "start"), checker.GreaterOrEqualThan, 2, check.Commentf("should have had 2 start events but had %d, out: %s", count, out))

}

func (s *DockerSuite) TestEventsFilterImageName(c *check.C) {
	testRequires(c, DaemonIsLinux)
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
	testRequires(c, DaemonIsLinux)
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
	testRequires(c, DaemonIsLinux)
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
		"--filter", fmt.Sprintf("label=%s", label))

	events := strings.Split(strings.TrimSpace(out), "\n")

	// 2 events from the "docker tag" command, another one is from "docker build"
	c.Assert(events, checker.HasLen, 3, check.Commentf("Events == %s", events))
	for _, e := range events {
		c.Assert(e, checker.Contains, "labelfiltertest")
	}
}

func (s *DockerSuite) TestEventsFilterContainer(c *check.C) {
	testRequires(c, DaemonIsLinux)
	since := fmt.Sprintf("%d", daemonTime(c).Unix())
	nameID := make(map[string]string)

	for _, name := range []string{"container_1", "container_2"} {
		dockerCmd(c, "run", "--name", name, "busybox", "true")
		id, err := inspectField(name, "Id")
		c.Assert(err, checker.IsNil)
		nameID[name] = id
	}

	until := fmt.Sprintf("%d", daemonTime(c).Unix())

	checkEvents := func(id string, events []string) error {
		if len(events) != 4 { // create, attach, start, die
			return fmt.Errorf("expected 4 events, got %v", events)
		}
		for _, event := range events {
			e := strings.Fields(event)
			if len(e) < 3 {
				return fmt.Errorf("got malformed event: %s", event)
			}

			// Check the id
			parsedID := strings.TrimSuffix(e[1], ":")
			if parsedID != id {
				return fmt.Errorf("expected event for container id %s: %s - parsed container id: %s", id, event, parsedID)
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

func (s *DockerSuite) TestEventsStreaming(c *check.C) {
	testRequires(c, DaemonIsLinux)
	start := daemonTime(c).Unix()

	id := make(chan string)
	eventCreate := make(chan struct{})
	eventStart := make(chan struct{})
	eventDie := make(chan struct{})
	eventDestroy := make(chan struct{})

	eventsCmd := exec.Command(dockerBinary, "events", "--since", strconv.FormatInt(start, 10))
	stdout, err := eventsCmd.StdoutPipe()
	c.Assert(err, checker.IsNil)
	c.Assert(eventsCmd.Start(), checker.IsNil, check.Commentf("failed to start 'docker events'"))
	defer eventsCmd.Process.Kill()

	go func() {
		containerID := <-id

		matchCreate := regexp.MustCompile(containerID + `: \(from busybox:latest\) create$`)
		matchStart := regexp.MustCompile(containerID + `: \(from busybox:latest\) start$`)
		matchDie := regexp.MustCompile(containerID + `: \(from busybox:latest\) die$`)
		matchDestroy := regexp.MustCompile(containerID + `: \(from busybox:latest\) destroy$`)

		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			switch {
			case matchCreate.MatchString(scanner.Text()):
				close(eventCreate)
			case matchStart.MatchString(scanner.Text()):
				close(eventStart)
			case matchDie.MatchString(scanner.Text()):
				close(eventDie)
			case matchDestroy.MatchString(scanner.Text()):
				close(eventDestroy)
			}
		}
	}()

	out, _ := dockerCmd(c, "run", "-d", "busybox:latest", "true")
	cleanedContainerID := strings.TrimSpace(out)
	id <- cleanedContainerID

	select {
	case <-time.After(5 * time.Second):
		c.Fatal("failed to observe container create in timely fashion")
	case <-eventCreate:
		// ignore, done
	}

	select {
	case <-time.After(5 * time.Second):
		c.Fatal("failed to observe container start in timely fashion")
	case <-eventStart:
		// ignore, done
	}

	select {
	case <-time.After(5 * time.Second):
		c.Fatal("failed to observe container die in timely fashion")
	case <-eventDie:
		// ignore, done
	}

	dockerCmd(c, "rm", cleanedContainerID)

	select {
	case <-time.After(5 * time.Second):
		c.Fatal("failed to observe container destroy in timely fashion")
	case <-eventDestroy:
		// ignore, done
	}
}

func (s *DockerSuite) TestEventsCommit(c *check.C) {
	testRequires(c, DaemonIsLinux)
	since := daemonTime(c).Unix()

	out, _ := dockerCmd(c, "run", "-d", "busybox", "top")
	cID := strings.TrimSpace(out)
	c.Assert(waitRun(cID), checker.IsNil)

	dockerCmd(c, "commit", "-m", "test", cID)
	dockerCmd(c, "stop", cID)

	out, _ = dockerCmd(c, "events", "--since=0", "-f", "container="+cID, "--until="+strconv.Itoa(int(since)))
	c.Assert(out, checker.Contains, " commit\n", check.Commentf("Missing 'commit' log event"))
}

func (s *DockerSuite) TestEventsCopy(c *check.C) {
	testRequires(c, DaemonIsLinux)
	since := daemonTime(c).Unix()

	// Build a test image.
	id, err := buildImage("cpimg", `
		  FROM busybox
		  RUN echo HI > /tmp/file`, true)
	c.Assert(err, checker.IsNil, check.Commentf("Couldn't create image"))

	// Create an empty test file.
	tempFile, err := ioutil.TempFile("", "test-events-copy-")
	c.Assert(err, checker.IsNil)
	defer os.Remove(tempFile.Name())

	c.Assert(tempFile.Close(), checker.IsNil)

	dockerCmd(c, "create", "--name=cptest", id)

	dockerCmd(c, "cp", "cptest:/tmp/file", tempFile.Name())

	out, _ := dockerCmd(c, "events", "--since=0", "-f", "container=cptest", "--until="+strconv.Itoa(int(since)))
	c.Assert(out, checker.Contains, " archive-path\n", check.Commentf("Missing 'archive-path' log event\n"))

	dockerCmd(c, "cp", tempFile.Name(), "cptest:/tmp/filecopy")

	out, _ = dockerCmd(c, "events", "--since=0", "-f", "container=cptest", "--until="+strconv.Itoa(int(since)))
	c.Assert(out, checker.Contains, " extract-to-dir\n", check.Commentf("Missing 'extract-to-dir' log event"))
}

func (s *DockerSuite) TestEventsResize(c *check.C) {
	testRequires(c, DaemonIsLinux)
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
	c.Assert(out, checker.Contains, " resize\n", check.Commentf("Missing 'resize' log event"))
}

func (s *DockerSuite) TestEventsAttach(c *check.C) {
	testRequires(c, DaemonIsLinux)
	since := daemonTime(c).Unix()

	out, _ := dockerCmd(c, "run", "-di", "busybox", "/bin/cat")
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

	dockerCmd(c, "stop", cID)

	out, _ = dockerCmd(c, "events", "--since=0", "-f", "container="+cID, "--until="+strconv.Itoa(int(since)))
	c.Assert(out, checker.Contains, " attach\n", check.Commentf("Missing 'attach' log event"))
}

func (s *DockerSuite) TestEventsRename(c *check.C) {
	testRequires(c, DaemonIsLinux)
	since := daemonTime(c).Unix()

	dockerCmd(c, "run", "--name", "oldName", "busybox", "true")
	dockerCmd(c, "rename", "oldName", "newName")

	out, _ := dockerCmd(c, "events", "--since=0", "-f", "container=newName", "--until="+strconv.Itoa(int(since)))
	c.Assert(out, checker.Contains, " rename\n", check.Commentf("Missing 'rename' log event\n"))
}

func (s *DockerSuite) TestEventsTop(c *check.C) {
	testRequires(c, DaemonIsLinux)
	since := daemonTime(c).Unix()

	out, _ := dockerCmd(c, "run", "-d", "busybox", "top")
	cID := strings.TrimSpace(out)
	c.Assert(waitRun(cID), checker.IsNil)

	dockerCmd(c, "top", cID)
	dockerCmd(c, "stop", cID)

	out, _ = dockerCmd(c, "events", "--since=0", "-f", "container="+cID, "--until="+strconv.Itoa(int(since)))
	c.Assert(out, checker.Contains, " top\n", check.Commentf("Missing 'top' log event"))
}

// #13753
func (s *DockerSuite) TestEventsDefaultEmpty(c *check.C) {
	testRequires(c, DaemonIsLinux)
	dockerCmd(c, "run", "busybox")
	out, _ := dockerCmd(c, "events", fmt.Sprintf("--until=%d", daemonTime(c).Unix()))
	c.Assert(strings.TrimSpace(out), checker.Equals, "")
}

// #14316
func (s *DockerRegistrySuite) TestEventsImageFilterPush(c *check.C) {
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
	c.Assert(out, checker.Contains, repoName+": push\n", check.Commentf("Missing 'push' log event"))
}
