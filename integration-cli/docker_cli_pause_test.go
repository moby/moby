package main

import (
	"fmt"
	"os/exec"
	"strings"
	"testing"
)

func TestPause(t *testing.T) {
	defer deleteAllContainers()
	defer unpauseAllContainers()

	name := "testeventpause"
	out, _, _ := dockerCmd(t, "images", "-q")
	image := strings.Split(out, "\n")[0]
	dockerCmd(t, "run", "-d", "--name", name, image, "sleep", "2")

	dockerCmd(t, "pause", name)
	pausedContainers, err := getSliceOfPausedContainers()
	if err != nil {
		t.Fatalf("error thrown while checking if containers were paused: %v", err)
	}
	if len(pausedContainers) != 1 {
		t.Fatalf("there should be one paused container and not", len(pausedContainers))
	}

	dockerCmd(t, "unpause", name)

	eventsCmd := exec.Command(dockerBinary, "events", "--since=0", fmt.Sprintf("--until=%d", daemonTime(t).Unix()))
	out, _, _ = runCommandWithOutput(eventsCmd)
	events := strings.Split(out, "\n")
	if len(events) <= 1 {
		t.Fatalf("Missing expected event")
	}

	pauseEvent := strings.Fields(events[len(events)-3])
	unpauseEvent := strings.Fields(events[len(events)-2])

	if pauseEvent[len(pauseEvent)-1] != "pause" {
		t.Fatalf("event should be pause, not %#v", pauseEvent)
	}
	if unpauseEvent[len(unpauseEvent)-1] != "unpause" {
		t.Fatalf("event should be unpause, not %#v", unpauseEvent)
	}

	waitCmd := exec.Command(dockerBinary, "wait", name)
	if waitOut, _, err := runCommandWithOutput(waitCmd); err != nil {
		t.Fatalf("error thrown while waiting for container: %s, %v", waitOut, err)
	}

	logDone("pause - pause/unpause is logged")
}

func TestPauseMultipleContainers(t *testing.T) {
	defer deleteAllContainers()
	defer unpauseAllContainers()

	containers := []string{
		"testpausewithmorecontainers1",
		"testpausewithmorecontainers2",
	}
	out, _, _ := dockerCmd(t, "images", "-q")
	image := strings.Split(out, "\n")[0]
	for _, name := range containers {
		dockerCmd(t, "run", "-d", "--name", name, image, "sleep", "2")
	}
	dockerCmd(t, append([]string{"pause"}, containers...)...)
	pausedContainers, err := getSliceOfPausedContainers()
	if err != nil {
		t.Fatalf("error thrown while checking if containers were paused: %v", err)
	}
	if len(pausedContainers) != len(containers) {
		t.Fatalf("there should be %d paused container and not %d", len(containers), len(pausedContainers))
	}

	dockerCmd(t, append([]string{"unpause"}, containers...)...)

	eventsCmd := exec.Command(dockerBinary, "events", "--since=0", fmt.Sprintf("--until=%d", daemonTime(t).Unix()))
	out, _, _ = runCommandWithOutput(eventsCmd)
	events := strings.Split(out, "\n")
	if len(events) <= len(containers)*3-2 {
		t.Fatalf("Missing expected event")
	}

	pauseEvents := make([][]string, len(containers))
	unpauseEvents := make([][]string, len(containers))
	for i := range containers {
		pauseEvents[i] = strings.Fields(events[len(events)-len(containers)*2-1+i])
		unpauseEvents[i] = strings.Fields(events[len(events)-len(containers)-1+i])
	}

	for _, pauseEvent := range pauseEvents {
		if pauseEvent[len(pauseEvent)-1] != "pause" {
			t.Fatalf("event should be pause, not %#v", pauseEvent)
		}
	}
	for _, unpauseEvent := range unpauseEvents {
		if unpauseEvent[len(unpauseEvent)-1] != "unpause" {
			t.Fatalf("event should be unpause, not %#v", unpauseEvent)
		}
	}

	for _, name := range containers {
		waitCmd := exec.Command(dockerBinary, "wait", name)
		if waitOut, _, err := runCommandWithOutput(waitCmd); err != nil {
			t.Fatalf("error thrown while waiting for container: %s, %v", waitOut, err)
		}
	}

	logDone("pause - multi pause/unpause is logged")
}
