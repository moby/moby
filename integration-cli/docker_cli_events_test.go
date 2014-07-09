package main

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestCLIGetEventsUntag(t *testing.T) {
	out, _, _ := cmd(t, "images", "-q")
	image := strings.Split(out, "\n")[0]
	cmd(t, "tag", image, "utest:tag1")
	cmd(t, "tag", image, "utest:tag2")
	cmd(t, "rmi", "utest:tag1")
	cmd(t, "rmi", "utest:tag2")
	eventsCmd := exec.Command("timeout", "0.2", dockerBinary, "events", "--since=1")
	out, _, _ = runCommandWithOutput(eventsCmd)
	events := strings.Split(out, "\n")
	n_events := len(events)
	// The last element after the split above will be an empty string, so we
	// get the two elements before the last, which are the untags we're
	// looking for.
	for _, v := range events[n_events-3 : n_events-1] {
		if !strings.Contains(v, "untag") {
			t.Fatalf("event should be untag, not %#v", v)
		}
	}
	logDone("events - untags are logged")
}

func TestCLIGetEventsPause(t *testing.T) {
	out, _, _ := cmd(t, "images", "-q")
	image := strings.Split(out, "\n")[0]
	cmd(t, "run", "-d", "--name", "testeventpause", image, "sleep", "2")
	cmd(t, "pause", "testeventpause")
	cmd(t, "unpause", "testeventpause")
	eventsCmd := exec.Command(dockerBinary, "events", "--since=0", fmt.Sprintf("--until=%d", time.Now().Unix()))
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
		t.Fatalf("event should be pause, not %#v", unpauseEvent)
	}

	logDone("events - pause/unpause is logged")
}

func TestCLILimitEvents(t *testing.T) {
	for i := 0; i < 30; i++ {
		cmd(t, "run", "busybox", "echo", strconv.Itoa(i))
	}
	eventsCmd := exec.Command(dockerBinary, "events", "--since=0", fmt.Sprintf("--until=%d", time.Now().Unix()))
	out, _, _ := runCommandWithOutput(eventsCmd)
	events := strings.Split(out, "\n")
	n_events := len(events) - 1
	if n_events != 64 {
		t.Fatalf("events should be limited to 64, but received %d", n_events)
	}
	logDone("events - limited to 64 entries")
}
