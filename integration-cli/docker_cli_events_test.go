package main

import (
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestEventsUntag(t *testing.T) {
	image := "busybox"
	dockerCmd(t, "tag", image, "utest:tag1")
	dockerCmd(t, "tag", image, "utest:tag2")
	dockerCmd(t, "rmi", "utest:tag1")
	dockerCmd(t, "rmi", "utest:tag2")
	eventsCmd := exec.Command(dockerBinary, "events", "--since=1")
	out, exitCode, _, err := runCommandWithOutputForDuration(eventsCmd, time.Duration(time.Millisecond*200))
	if exitCode != 0 || err != nil {
		t.Fatalf("Failed to get events - exit code %d: %s", exitCode, err)
	}
	events := strings.Split(out, "\n")
	nEvents := len(events)
	// The last element after the split above will be an empty string, so we
	// get the two elements before the last, which are the untags we're
	// looking for.
	for _, v := range events[nEvents-3 : nEvents-1] {
		if !strings.Contains(v, "untag") {
			t.Fatalf("event should be untag, not %#v", v)
		}
	}
	logDone("events - untags are logged")
}

func TestEventsContainerFailStartDie(t *testing.T) {
	defer deleteAllContainers()

	out, _, _ := dockerCmd(t, "images", "-q")
	image := strings.Split(out, "\n")[0]
	eventsCmd := exec.Command(dockerBinary, "run", "--name", "testeventdie", image, "blerg")
	_, _, err := runCommandWithOutput(eventsCmd)
	if err == nil {
		t.Fatalf("Container run with command blerg should have failed, but it did not")
	}

	eventsCmd = exec.Command(dockerBinary, "events", "--since=0", fmt.Sprintf("--until=%d", time.Now().Unix()))
	out, _, _ = runCommandWithOutput(eventsCmd)
	events := strings.Split(out, "\n")
	if len(events) <= 1 {
		t.Fatalf("Missing expected event")
	}

	startEvent := strings.Fields(events[len(events)-3])
	dieEvent := strings.Fields(events[len(events)-2])

	if startEvent[len(startEvent)-1] != "start" {
		t.Fatalf("event should be start, not %#v", startEvent)
	}
	if dieEvent[len(dieEvent)-1] != "die" {
		t.Fatalf("event should be die, not %#v", dieEvent)
	}

	logDone("events - container unwilling to start logs die")
}

func TestEventsLimit(t *testing.T) {
	defer deleteAllContainers()
	for i := 0; i < 30; i++ {
		dockerCmd(t, "run", "busybox", "echo", strconv.Itoa(i))
	}
	eventsCmd := exec.Command(dockerBinary, "events", "--since=0", fmt.Sprintf("--until=%d", time.Now().Unix()))
	out, _, _ := runCommandWithOutput(eventsCmd)
	events := strings.Split(out, "\n")
	nEvents := len(events) - 1
	if nEvents != 64 {
		t.Fatalf("events should be limited to 64, but received %d", nEvents)
	}
	logDone("events - limited to 64 entries")
}

func TestEventsContainerEvents(t *testing.T) {
	dockerCmd(t, "run", "--rm", "busybox", "true")
	eventsCmd := exec.Command(dockerBinary, "events", "--since=0", fmt.Sprintf("--until=%d", time.Now().Unix()))
	out, exitCode, err := runCommandWithOutput(eventsCmd)
	if exitCode != 0 || err != nil {
		t.Fatalf("Failed to get events with exit code %d: %s", exitCode, err)
	}
	events := strings.Split(out, "\n")
	events = events[:len(events)-1]
	if len(events) < 4 {
		t.Fatalf("Missing expected event")
	}
	createEvent := strings.Fields(events[len(events)-4])
	startEvent := strings.Fields(events[len(events)-3])
	dieEvent := strings.Fields(events[len(events)-2])
	destroyEvent := strings.Fields(events[len(events)-1])
	if createEvent[len(createEvent)-1] != "create" {
		t.Fatalf("event should be create, not %#v", createEvent)
	}
	if startEvent[len(startEvent)-1] != "start" {
		t.Fatalf("event should be start, not %#v", startEvent)
	}
	if dieEvent[len(dieEvent)-1] != "die" {
		t.Fatalf("event should be die, not %#v", dieEvent)
	}
	if destroyEvent[len(destroyEvent)-1] != "destroy" {
		t.Fatalf("event should be destroy, not %#v", destroyEvent)
	}

	logDone("events - container create, start, die, destroy is logged")
}

func TestEventsImageUntagDelete(t *testing.T) {
	name := "testimageevents"
	defer deleteImages(name)
	_, err := buildImage(name,
		`FROM scratch
		MAINTAINER "docker"`,
		true)
	if err != nil {
		t.Fatal(err)
	}
	if err := deleteImages(name); err != nil {
		t.Fatal(err)
	}
	eventsCmd := exec.Command(dockerBinary, "events", "--since=0", fmt.Sprintf("--until=%d", time.Now().Unix()))
	out, exitCode, err := runCommandWithOutput(eventsCmd)
	if exitCode != 0 || err != nil {
		t.Fatalf("Failed to get events with exit code %d: %s", exitCode, err)
	}
	events := strings.Split(out, "\n")

	events = events[:len(events)-1]
	if len(events) < 2 {
		t.Fatalf("Missing expected event")
	}
	untagEvent := strings.Fields(events[len(events)-2])
	deleteEvent := strings.Fields(events[len(events)-1])
	if untagEvent[len(untagEvent)-1] != "untag" {
		t.Fatalf("untag should be untag, not %#v", untagEvent)
	}
	if deleteEvent[len(deleteEvent)-1] != "delete" {
		t.Fatalf("delete should be delete, not %#v", deleteEvent)
	}
	logDone("events - image untag, delete is logged")
}

func TestEventsImagePull(t *testing.T) {
	since := time.Now().Unix()

	defer deleteImages("hello-world")

	pullCmd := exec.Command(dockerBinary, "pull", "hello-world")
	if out, _, err := runCommandWithOutput(pullCmd); err != nil {
		t.Fatalf("pulling the hello-world image from has failed: %s, %v", out, err)
	}

	eventsCmd := exec.Command(dockerBinary, "events",
		fmt.Sprintf("--since=%d", since),
		fmt.Sprintf("--until=%d", time.Now().Unix()))
	out, _, _ := runCommandWithOutput(eventsCmd)

	events := strings.Split(strings.TrimSpace(out), "\n")
	event := strings.TrimSpace(events[len(events)-1])

	if !strings.HasSuffix(event, "hello-world:latest: pull") {
		t.Fatalf("Missing pull event - got:%q", event)
	}

	logDone("events - image pull is logged")
}

func TestEventsImageImport(t *testing.T) {
	defer deleteAllContainers()
	since := time.Now().Unix()

	runCmd := exec.Command(dockerBinary, "run", "-d", "busybox", "true")
	out, _, err := runCommandWithOutput(runCmd)
	if err != nil {
		t.Fatal("failed to create a container", out, err)
	}
	cleanedContainerID := stripTrailingCharacters(out)

	out, _, err = runCommandPipelineWithOutput(
		exec.Command(dockerBinary, "export", cleanedContainerID),
		exec.Command(dockerBinary, "import", "-"),
	)
	if err != nil {
		t.Errorf("import failed with errors: %v, output: %q", err, out)
	}

	eventsCmd := exec.Command(dockerBinary, "events",
		fmt.Sprintf("--since=%d", since),
		fmt.Sprintf("--until=%d", time.Now().Unix()))
	out, _, _ = runCommandWithOutput(eventsCmd)

	events := strings.Split(strings.TrimSpace(out), "\n")
	event := strings.TrimSpace(events[len(events)-1])

	if !strings.HasSuffix(event, ": import") {
		t.Fatalf("Missing pull event - got:%q", event)
	}

	logDone("events - image import is logged")
}

func TestEventsFilters(t *testing.T) {
	parseEvents := func(out, match string) {
		events := strings.Split(out, "\n")
		events = events[:len(events)-1]
		for _, event := range events {
			eventFields := strings.Fields(event)
			eventName := eventFields[len(eventFields)-1]
			if ok, err := regexp.MatchString(match, eventName); err != nil || !ok {
				t.Fatalf("event should match %s, got %#v, err: %v", match, eventFields, err)
			}
		}
	}

	since := time.Now().Unix()
	out, _, err := runCommandWithOutput(exec.Command(dockerBinary, "run", "--rm", "busybox", "true"))
	if err != nil {
		t.Fatal(out, err)
	}
	out, _, err = runCommandWithOutput(exec.Command(dockerBinary, "run", "--rm", "busybox", "true"))
	if err != nil {
		t.Fatal(out, err)
	}
	out, _, err = runCommandWithOutput(exec.Command(dockerBinary, "events", fmt.Sprintf("--since=%d", since), fmt.Sprintf("--until=%d", time.Now().Unix()), "--filter", "event=die"))
	if err != nil {
		t.Fatalf("Failed to get events: %s", err)
	}
	parseEvents(out, "die")

	out, _, err = runCommandWithOutput(exec.Command(dockerBinary, "events", fmt.Sprintf("--since=%d", since), fmt.Sprintf("--until=%d", time.Now().Unix()), "--filter", "event=die", "--filter", "event=start"))
	if err != nil {
		t.Fatalf("Failed to get events: %s", err)
	}
	parseEvents(out, "((die)|(start))")

	// make sure we at least got 2 start events
	count := strings.Count(out, "start")
	if count < 2 {
		t.Fatalf("should have had 2 start events but had %d, out: %s", count, out)
	}

	logDone("events - filters")
}
