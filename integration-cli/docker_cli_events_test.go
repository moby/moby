package main

import (
	"os/exec"
	"strings"
	"testing"
)

func TestCLIGetEvents(t *testing.T) {
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
