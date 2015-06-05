package main

import (
	"bufio"
	"fmt"
	"net/http"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-check/check"
)

func (s *DockerSuite) TestEventsTimestampFormats(c *check.C) {
	image := "busybox"

	// Start stopwatch, generate an event
	time.Sleep(time.Second) // so that we don't grab events from previous test occured in the same second
	start := daemonTime(c)
	time.Sleep(time.Second) // remote API precision is only a second, wait a while before creating an event
	dockerCmd(c, "tag", image, "timestamptest:1")
	dockerCmd(c, "rmi", "timestamptest:1")
	time.Sleep(time.Second) // so that until > since
	end := daemonTime(c)

	// List of available time formats to --since
	unixTs := func(t time.Time) string { return fmt.Sprintf("%v", t.Unix()) }
	rfc3339 := func(t time.Time) string { return t.Format(time.RFC3339) }
	duration := func(t time.Time) string { return time.Now().Sub(t).String() }

	// --since=$start must contain only the 'untag' event
	for _, f := range []func(time.Time) string{unixTs, rfc3339, duration} {
		since, until := f(start), f(end)
		cmd := exec.Command(dockerBinary, "events", "--since="+since, "--until="+until)
		out, _, err := runCommandWithOutput(cmd)
		if err != nil {
			c.Fatalf("docker events cmd failed: %v\nout=%s", err, out)
		}
		events := strings.Split(strings.TrimSpace(out), "\n")
		if len(events) != 2 {
			c.Fatalf("unexpected events, was expecting only 2 events tag/untag (since=%s, until=%s) out=%s", since, until, out)
		}
		if !strings.Contains(out, "untag") {
			c.Fatalf("expected 'untag' event not found (since=%s, until=%s) out=%s", since, until, out)
		}
	}

}

func (s *DockerSuite) TestEventsUntag(c *check.C) {
	image := "busybox"
	dockerCmd(c, "tag", image, "utest:tag1")
	dockerCmd(c, "tag", image, "utest:tag2")
	dockerCmd(c, "rmi", "utest:tag1")
	dockerCmd(c, "rmi", "utest:tag2")
	eventsCmd := exec.Command(dockerBinary, "events", "--since=1")
	out, exitCode, _, err := runCommandWithOutputForDuration(eventsCmd, time.Duration(time.Millisecond*200))
	if exitCode != 0 || err != nil {
		c.Fatalf("Failed to get events - exit code %d: %s", exitCode, err)
	}
	events := strings.Split(out, "\n")
	nEvents := len(events)
	// The last element after the split above will be an empty string, so we
	// get the two elements before the last, which are the untags we're
	// looking for.
	for _, v := range events[nEvents-3 : nEvents-1] {
		if !strings.Contains(v, "untag") {
			c.Fatalf("event should be untag, not %#v", v)
		}
	}
}

func (s *DockerSuite) TestEventsContainerFailStartDie(c *check.C) {

	out, _ := dockerCmd(c, "images", "-q")
	image := strings.Split(out, "\n")[0]
	if err := exec.Command(dockerBinary, "run", "--name", "testeventdie", image, "blerg").Run(); err == nil {
		c.Fatalf("Container run with command blerg should have failed, but it did not")
	}

	eventsCmd := exec.Command(dockerBinary, "events", "--since=0", fmt.Sprintf("--until=%d", daemonTime(c).Unix()))
	out, _, _ = runCommandWithOutput(eventsCmd)
	events := strings.Split(out, "\n")
	if len(events) <= 1 {
		c.Fatalf("Missing expected event")
	}

	startEvent := strings.Fields(events[len(events)-3])
	dieEvent := strings.Fields(events[len(events)-2])

	if startEvent[len(startEvent)-1] != "start" {
		c.Fatalf("event should be start, not %#v", startEvent)
	}
	if dieEvent[len(dieEvent)-1] != "die" {
		c.Fatalf("event should be die, not %#v", dieEvent)
	}

}

func (s *DockerSuite) TestEventsLimit(c *check.C) {

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
		if err != nil {
			c.Fatalf("%q failed with error: %v", strings.Join(args, " "), err)
		}
	}

	eventsCmd := exec.Command(dockerBinary, "events", "--since=0", fmt.Sprintf("--until=%d", daemonTime(c).Unix()))
	out, _, _ := runCommandWithOutput(eventsCmd)
	events := strings.Split(out, "\n")
	nEvents := len(events) - 1
	if nEvents != 64 {
		c.Fatalf("events should be limited to 64, but received %d", nEvents)
	}
}

func (s *DockerSuite) TestEventsContainerEvents(c *check.C) {
	dockerCmd(c, "run", "--rm", "busybox", "true")
	eventsCmd := exec.Command(dockerBinary, "events", "--since=0", fmt.Sprintf("--until=%d", daemonTime(c).Unix()))
	out, exitCode, err := runCommandWithOutput(eventsCmd)
	if exitCode != 0 || err != nil {
		c.Fatalf("Failed to get events with exit code %d: %s", exitCode, err)
	}
	events := strings.Split(out, "\n")
	events = events[:len(events)-1]
	if len(events) < 5 {
		c.Fatalf("Missing expected event")
	}
	createEvent := strings.Fields(events[len(events)-5])
	attachEvent := strings.Fields(events[len(events)-4])
	startEvent := strings.Fields(events[len(events)-3])
	dieEvent := strings.Fields(events[len(events)-2])
	destroyEvent := strings.Fields(events[len(events)-1])
	if createEvent[len(createEvent)-1] != "create" {
		c.Fatalf("event should be create, not %#v", createEvent)
	}
	if attachEvent[len(createEvent)-1] != "attach" {
		c.Fatalf("event should be attach, not %#v", attachEvent)
	}
	if startEvent[len(startEvent)-1] != "start" {
		c.Fatalf("event should be start, not %#v", startEvent)
	}
	if dieEvent[len(dieEvent)-1] != "die" {
		c.Fatalf("event should be die, not %#v", dieEvent)
	}
	if destroyEvent[len(destroyEvent)-1] != "destroy" {
		c.Fatalf("event should be destroy, not %#v", destroyEvent)
	}

}

func (s *DockerSuite) TestEventsContainerEventsSinceUnixEpoch(c *check.C) {
	dockerCmd(c, "run", "--rm", "busybox", "true")
	timeBeginning := time.Unix(0, 0).Format(time.RFC3339Nano)
	timeBeginning = strings.Replace(timeBeginning, "Z", ".000000000Z", -1)
	eventsCmd := exec.Command(dockerBinary, "events", fmt.Sprintf("--since='%s'", timeBeginning),
		fmt.Sprintf("--until=%d", daemonTime(c).Unix()))
	out, exitCode, err := runCommandWithOutput(eventsCmd)
	if exitCode != 0 || err != nil {
		c.Fatalf("Failed to get events with exit code %d: %s", exitCode, err)
	}
	events := strings.Split(out, "\n")
	events = events[:len(events)-1]
	if len(events) < 5 {
		c.Fatalf("Missing expected event")
	}
	createEvent := strings.Fields(events[len(events)-5])
	attachEvent := strings.Fields(events[len(events)-4])
	startEvent := strings.Fields(events[len(events)-3])
	dieEvent := strings.Fields(events[len(events)-2])
	destroyEvent := strings.Fields(events[len(events)-1])
	if createEvent[len(createEvent)-1] != "create" {
		c.Fatalf("event should be create, not %#v", createEvent)
	}
	if attachEvent[len(attachEvent)-1] != "attach" {
		c.Fatalf("event should be attach, not %#v", attachEvent)
	}
	if startEvent[len(startEvent)-1] != "start" {
		c.Fatalf("event should be start, not %#v", startEvent)
	}
	if dieEvent[len(dieEvent)-1] != "die" {
		c.Fatalf("event should be die, not %#v", dieEvent)
	}
	if destroyEvent[len(destroyEvent)-1] != "destroy" {
		c.Fatalf("event should be destroy, not %#v", destroyEvent)
	}

}

func (s *DockerSuite) TestEventsImageUntagDelete(c *check.C) {
	name := "testimageevents"
	_, err := buildImage(name,
		`FROM scratch
		MAINTAINER "docker"`,
		true)
	if err != nil {
		c.Fatal(err)
	}
	if err := deleteImages(name); err != nil {
		c.Fatal(err)
	}
	eventsCmd := exec.Command(dockerBinary, "events", "--since=0", fmt.Sprintf("--until=%d", daemonTime(c).Unix()))
	out, exitCode, err := runCommandWithOutput(eventsCmd)
	if exitCode != 0 || err != nil {
		c.Fatalf("Failed to get events with exit code %d: %s", exitCode, err)
	}
	events := strings.Split(out, "\n")

	events = events[:len(events)-1]
	if len(events) < 2 {
		c.Fatalf("Missing expected event")
	}
	untagEvent := strings.Fields(events[len(events)-2])
	deleteEvent := strings.Fields(events[len(events)-1])
	if untagEvent[len(untagEvent)-1] != "untag" {
		c.Fatalf("untag should be untag, not %#v", untagEvent)
	}
	if deleteEvent[len(deleteEvent)-1] != "delete" {
		c.Fatalf("delete should be delete, not %#v", deleteEvent)
	}
}

func (s *DockerSuite) TestEventsImageTag(c *check.C) {
	time.Sleep(time.Second * 2) // because API has seconds granularity
	since := daemonTime(c).Unix()
	image := "testimageevents:tag"
	dockerCmd(c, "tag", "busybox", image)

	eventsCmd := exec.Command(dockerBinary, "events",
		fmt.Sprintf("--since=%d", since),
		fmt.Sprintf("--until=%d", daemonTime(c).Unix()))
	out, _, err := runCommandWithOutput(eventsCmd)
	c.Assert(err, check.IsNil)

	events := strings.Split(strings.TrimSpace(out), "\n")
	if len(events) != 1 {
		c.Fatalf("was expecting 1 event. out=%s", out)
	}
	event := strings.TrimSpace(events[0])
	expectedStr := image + ": tag"

	if !strings.HasSuffix(event, expectedStr) {
		c.Fatalf("wrong event format. expected='%s' got=%s", expectedStr, event)
	}

}

func (s *DockerSuite) TestEventsImagePull(c *check.C) {
	since := daemonTime(c).Unix()
	testRequires(c, Network)

	pullCmd := exec.Command(dockerBinary, "pull", "hello-world")
	if out, _, err := runCommandWithOutput(pullCmd); err != nil {
		c.Fatalf("pulling the hello-world image from has failed: %s, %v", out, err)
	}

	eventsCmd := exec.Command(dockerBinary, "events",
		fmt.Sprintf("--since=%d", since),
		fmt.Sprintf("--until=%d", daemonTime(c).Unix()))
	out, _, _ := runCommandWithOutput(eventsCmd)

	events := strings.Split(strings.TrimSpace(out), "\n")
	event := strings.TrimSpace(events[len(events)-1])

	if !strings.HasSuffix(event, "hello-world:latest: pull") {
		c.Fatalf("Missing pull event - got:%q", event)
	}

}

func (s *DockerSuite) TestEventsImageImport(c *check.C) {
	since := daemonTime(c).Unix()

	id := make(chan string)
	eventImport := make(chan struct{})
	eventsCmd := exec.Command(dockerBinary, "events", "--since", strconv.FormatInt(since, 10))
	stdout, err := eventsCmd.StdoutPipe()
	if err != nil {
		c.Fatal(err)
	}
	if err := eventsCmd.Start(); err != nil {
		c.Fatal(err)
	}
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

	runCmd := exec.Command(dockerBinary, "run", "-d", "busybox", "true")
	out, _, err := runCommandWithOutput(runCmd)
	if err != nil {
		c.Fatal("failed to create a container", out, err)
	}
	cleanedContainerID := strings.TrimSpace(out)

	out, _, err = runCommandPipelineWithOutput(
		exec.Command(dockerBinary, "export", cleanedContainerID),
		exec.Command(dockerBinary, "import", "-"),
	)
	if err != nil {
		c.Errorf("import failed with errors: %v, output: %q", err, out)
	}
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
	parseEvents := func(out, match string) {
		events := strings.Split(out, "\n")
		events = events[:len(events)-1]
		for _, event := range events {
			eventFields := strings.Fields(event)
			eventName := eventFields[len(eventFields)-1]
			if ok, err := regexp.MatchString(match, eventName); err != nil || !ok {
				c.Fatalf("event should match %s, got %#v, err: %v", match, eventFields, err)
			}
		}
	}

	since := daemonTime(c).Unix()
	out, _, err := runCommandWithOutput(exec.Command(dockerBinary, "run", "--rm", "busybox", "true"))
	if err != nil {
		c.Fatal(out, err)
	}
	out, _, err = runCommandWithOutput(exec.Command(dockerBinary, "run", "--rm", "busybox", "true"))
	if err != nil {
		c.Fatal(out, err)
	}
	out, _, err = runCommandWithOutput(exec.Command(dockerBinary, "events", fmt.Sprintf("--since=%d", since), fmt.Sprintf("--until=%d", daemonTime(c).Unix()), "--filter", "event=die"))
	if err != nil {
		c.Fatalf("Failed to get events: %s", err)
	}
	parseEvents(out, "die")

	out, _, err = runCommandWithOutput(exec.Command(dockerBinary, "events", fmt.Sprintf("--since=%d", since), fmt.Sprintf("--until=%d", daemonTime(c).Unix()), "--filter", "event=die", "--filter", "event=start"))
	if err != nil {
		c.Fatalf("Failed to get events: %s", err)
	}
	parseEvents(out, "((die)|(start))")

	// make sure we at least got 2 start events
	count := strings.Count(out, "start")
	if count < 2 {
		c.Fatalf("should have had 2 start events but had %d, out: %s", count, out)
	}

}

func (s *DockerSuite) TestEventsFilterImageName(c *check.C) {
	since := daemonTime(c).Unix()

	out, _, err := runCommandWithOutput(exec.Command(dockerBinary, "run", "--name", "container_1", "-d", "busybox:latest", "true"))
	if err != nil {
		c.Fatal(out, err)
	}
	container1 := strings.TrimSpace(out)

	out, _, err = runCommandWithOutput(exec.Command(dockerBinary, "run", "--name", "container_2", "-d", "busybox", "true"))
	if err != nil {
		c.Fatal(out, err)
	}
	container2 := strings.TrimSpace(out)

	name := "busybox"
	eventsCmd := exec.Command(dockerBinary, "events", fmt.Sprintf("--since=%d", since), fmt.Sprintf("--until=%d", daemonTime(c).Unix()), "--filter", fmt.Sprintf("image=%s", name))
	out, _, err = runCommandWithOutput(eventsCmd)
	if err != nil {
		c.Fatalf("Failed to get events, error: %s(%s)", err, out)
	}
	events := strings.Split(out, "\n")
	events = events[:len(events)-1]
	if len(events) == 0 {
		c.Fatalf("Expected events but found none for the image busybox:latest")
	}
	count1 := 0
	count2 := 0

	for _, e := range events {
		if strings.Contains(e, container1) {
			count1++
		} else if strings.Contains(e, container2) {
			count2++
		}
	}
	if count1 == 0 || count2 == 0 {
		c.Fatalf("Expected events from each container but got %d from %s and %d from %s", count1, container1, count2, container2)
	}

}

func (s *DockerSuite) TestEventsFilterContainer(c *check.C) {
	since := fmt.Sprintf("%d", daemonTime(c).Unix())
	nameID := make(map[string]string)

	for _, name := range []string{"container_1", "container_2"} {
		out, _, err := runCommandWithOutput(exec.Command(dockerBinary, "run", "--name", name, "busybox", "true"))
		if err != nil {
			c.Fatalf("Error: %v, Output: %s", err, out)
		}
		id, err := inspectField(name, "Id")
		if err != nil {
			c.Fatal(err)
		}
		nameID[name] = id
	}

	until := fmt.Sprintf("%d", daemonTime(c).Unix())

	checkEvents := func(id string, events []string) error {
		if len(events) != 4 { // create, attach, start, die
			return fmt.Errorf("expected 3 events, got %v", events)
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
		eventsCmd := exec.Command(dockerBinary, "events", "--since", since, "--until", until, "--filter", "container="+name)
		out, _, err := runCommandWithOutput(eventsCmd)
		if err != nil {
			c.Fatal(err)
		}

		events := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
		if err := checkEvents(ID, events); err != nil {
			c.Fatal(err)
		}

		// filter by ID's
		eventsCmd = exec.Command(dockerBinary, "events", "--since", since, "--until", until, "--filter", "container="+ID)
		out, _, err = runCommandWithOutput(eventsCmd)
		if err != nil {
			c.Fatal(err)
		}

		events = strings.Split(strings.TrimSuffix(out, "\n"), "\n")
		if err := checkEvents(ID, events); err != nil {
			c.Fatal(err)
		}
	}

}

func (s *DockerSuite) TestEventsStreaming(c *check.C) {
	start := daemonTime(c).Unix()

	id := make(chan string)
	eventCreate := make(chan struct{})
	eventStart := make(chan struct{})
	eventDie := make(chan struct{})
	eventDestroy := make(chan struct{})

	eventsCmd := exec.Command(dockerBinary, "events", "--since", strconv.FormatInt(start, 10))
	stdout, err := eventsCmd.StdoutPipe()
	if err != nil {
		c.Fatal(err)
	}
	if err := eventsCmd.Start(); err != nil {
		c.Fatalf("failed to start 'docker events': %s", err)
	}
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

	runCmd := exec.Command(dockerBinary, "run", "-d", "busybox:latest", "true")
	out, _, err := runCommandWithOutput(runCmd)
	if err != nil {
		c.Fatal(out, err)
	}
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

	rmCmd := exec.Command(dockerBinary, "rm", cleanedContainerID)
	out, _, err = runCommandWithOutput(rmCmd)
	if err != nil {
		c.Fatal(out, err)
	}

	select {
	case <-time.After(5 * time.Second):
		c.Fatal("failed to observe container destroy in timely fashion")
	case <-eventDestroy:
		// ignore, done
	}
}

func (s *DockerSuite) TestEventsCommit(c *check.C) {
	since := daemonTime(c).Unix()

	runCmd := exec.Command(dockerBinary, "run", "-d", "busybox", "top")
	out, _, err := runCommandWithOutput(runCmd)
	if err != nil {
		c.Fatalf("Couldn't run top: %s\n%q", out, err)
	}
	cID := strings.TrimSpace(out)
	c.Assert(waitRun(cID), check.IsNil)

	cmd := exec.Command(dockerBinary, "commit", "-m", "test", cID)
	out, _, err = runCommandWithOutput(cmd)
	if err != nil {
		c.Fatalf("Couldn't commit: %s\n%q", out, err)
	}

	cmd = exec.Command(dockerBinary, "stop", cID)
	out, _, err = runCommandWithOutput(cmd)
	if err != nil {
		c.Fatalf("Couldn't stop: %s\n%q", out, err)
	}

	cmd = exec.Command(dockerBinary, "events", "--since=0", "-f", "container="+cID, "--until="+strconv.Itoa(int(since)))
	out, _, err = runCommandWithOutput(cmd)
	if err != nil {
		c.Fatalf("Couldn't get events: %s\n%q", out, err)
	}

	if !strings.Contains(out, " commit\n") {
		c.Fatalf("Missing 'commit' log event\n%s", out)
	}
}

func (s *DockerSuite) TestEventsCopy(c *check.C) {
	since := daemonTime(c).Unix()

	id, err := buildImage("cpimg", `
		  FROM busybox
		  RUN echo HI > /tmp/file`, true)
	if err != nil {
		c.Fatalf("Couldn't create image: %q", err)
	}

	runCmd := exec.Command(dockerBinary, "run", "--name=cptest", id, "true")
	out, _, err := runCommandWithOutput(runCmd)
	if err != nil {
		c.Fatalf("Couldn't run top: %s\n%q", out, err)
	}

	cmd := exec.Command(dockerBinary, "cp", "cptest:/tmp/file", "-")
	out, _, err = runCommandWithOutput(cmd)
	if err != nil {
		c.Fatalf("Failed getting file:%q\n%q", out, err)
	}

	cmd = exec.Command(dockerBinary, "events", "--since=0", "-f", "container=cptest", "--until="+strconv.Itoa(int(since)))
	out, _, err = runCommandWithOutput(cmd)
	if err != nil {
		c.Fatalf("Couldn't get events: %s\n%q", out, err)
	}

	if !strings.Contains(out, " copy\n") {
		c.Fatalf("Missing 'copy' log event\n%s", out)
	}
}

func (s *DockerSuite) TestEventsResize(c *check.C) {
	since := daemonTime(c).Unix()

	runCmd := exec.Command(dockerBinary, "run", "-d", "busybox", "top")
	out, _, err := runCommandWithOutput(runCmd)
	if err != nil {
		c.Fatalf("Couldn't run top: %s\n%q", out, err)
	}
	cID := strings.TrimSpace(out)
	c.Assert(waitRun(cID), check.IsNil)

	endpoint := "/containers/" + cID + "/resize?h=80&w=24"
	status, _, err := sockRequest("POST", endpoint, nil)
	c.Assert(status, check.Equals, http.StatusOK)
	c.Assert(err, check.IsNil)

	cmd := exec.Command(dockerBinary, "stop", cID)
	out, _, err = runCommandWithOutput(cmd)
	if err != nil {
		c.Fatalf("Couldn't stop: %s\n%q", out, err)
	}

	cmd = exec.Command(dockerBinary, "events", "--since=0", "-f", "container="+cID, "--until="+strconv.Itoa(int(since)))
	out, _, err = runCommandWithOutput(cmd)
	if err != nil {
		c.Fatalf("Couldn't get events: %s\n%q", out, err)
	}

	if !strings.Contains(out, " resize\n") {
		c.Fatalf("Missing 'resize' log event\n%s", out)
	}
}

func (s *DockerSuite) TestEventsAttach(c *check.C) {
	since := daemonTime(c).Unix()

	out, _ := dockerCmd(c, "run", "-di", "busybox", "/bin/cat")
	cID := strings.TrimSpace(out)

	cmd := exec.Command(dockerBinary, "attach", cID)
	stdin, err := cmd.StdinPipe()
	c.Assert(err, check.IsNil)
	defer stdin.Close()
	stdout, err := cmd.StdoutPipe()
	c.Assert(err, check.IsNil)
	defer stdout.Close()
	c.Assert(cmd.Start(), check.IsNil)
	defer cmd.Process.Kill()

	// Make sure we're done attaching by writing/reading some stuff
	if _, err := stdin.Write([]byte("hello\n")); err != nil {
		c.Fatal(err)
	}
	out, err = bufio.NewReader(stdout).ReadString('\n')
	c.Assert(err, check.IsNil)
	if strings.TrimSpace(out) != "hello" {
		c.Fatalf("expected 'hello', got %q", out)
	}

	c.Assert(stdin.Close(), check.IsNil)

	cmd = exec.Command(dockerBinary, "stop", cID)
	out, _, err = runCommandWithOutput(cmd)
	if err != nil {
		c.Fatalf("Couldn't stop: %s\n%q", out, err)
	}

	cmd = exec.Command(dockerBinary, "events", "--since=0", "-f", "container="+cID, "--until="+strconv.Itoa(int(since)))
	out, _, err = runCommandWithOutput(cmd)
	if err != nil {
		c.Fatalf("Couldn't get events: %s\n%q", out, err)
	}

	if !strings.Contains(out, " attach\n") {
		c.Fatalf("Missing 'attach' log event\n%s", out)
	}
}

func (s *DockerSuite) TestEventsRename(c *check.C) {
	since := daemonTime(c).Unix()

	runCmd := exec.Command(dockerBinary, "run", "--name", "oldName", "busybox", "true")
	out, _, err := runCommandWithOutput(runCmd)
	if err != nil {
		c.Fatalf("Couldn't run true: %s\n%q", out, err)
	}

	renameCmd := exec.Command(dockerBinary, "rename", "oldName", "newName")
	out, _, err = runCommandWithOutput(renameCmd)
	if err != nil {
		c.Fatalf("Couldn't rename: %s\n%q", out, err)
	}

	cmd := exec.Command(dockerBinary, "events", "--since=0", "-f", "container=newName", "--until="+strconv.Itoa(int(since)))
	out, _, err = runCommandWithOutput(cmd)
	if err != nil {
		c.Fatalf("Couldn't get events: %s\n%q", out, err)
	}

	if !strings.Contains(out, " rename\n") {
		c.Fatalf("Missing 'rename' log event\n%s", out)
	}
}

func (s *DockerSuite) TestEventsTop(c *check.C) {
	since := daemonTime(c).Unix()

	runCmd := exec.Command(dockerBinary, "run", "-d", "busybox", "top")
	out, _, err := runCommandWithOutput(runCmd)
	if err != nil {
		c.Fatalf("Couldn't run true: %s\n%q", out, err)
	}
	cID := strings.TrimSpace(out)
	c.Assert(waitRun(cID), check.IsNil)

	cmd := exec.Command(dockerBinary, "top", cID)
	out, _, err = runCommandWithOutput(cmd)
	if err != nil {
		c.Fatalf("Couldn't run docker top: %s\n%q", out, err)
	}

	cmd = exec.Command(dockerBinary, "stop", cID)
	out, _, err = runCommandWithOutput(cmd)
	if err != nil {
		c.Fatalf("Couldn't stop: %s\n%q", out, err)
	}

	cmd = exec.Command(dockerBinary, "events", "--since=0", "-f", "container="+cID, "--until="+strconv.Itoa(int(since)))
	out, _, err = runCommandWithOutput(cmd)
	if err != nil {
		c.Fatalf("Couldn't get events: %s\n%q", out, err)
	}

	if !strings.Contains(out, " top\n") {
		c.Fatalf("Missing 'top' log event\n%s", out)
	}
}

// #13753
func (s *DockerSuite) TestEventsDefaultEmpty(c *check.C) {
	dockerCmd(c, "run", "-d", "busybox")
	out, _ := dockerCmd(c, "events", fmt.Sprintf("--until=%d", daemonTime(c).Unix()))
	c.Assert(strings.TrimSpace(out), check.Equals, "")
}
