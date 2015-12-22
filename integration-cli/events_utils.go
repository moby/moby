package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"strconv"

	"github.com/go-check/check"
)

// eventMatcher is a function that tries to match an event input.
type eventMatcher func(text string)

// eventObserver runs an events commands and observes its output.
type eventObserver struct {
	buffer  *bytes.Buffer
	command *exec.Cmd
	stdout  io.Reader
}

// newEventObserver creates the observer and initializes the command
// without running it. Users must call `eventObserver.Start` to start the command.
func newEventObserver(c *check.C, args ...string) (*eventObserver, error) {
	since := daemonTime(c).Unix()

	cmdArgs := []string{"events", "--since", strconv.FormatInt(since, 10)}
	if len(args) > 0 {
		cmdArgs = append(cmdArgs, args...)
	}
	eventsCmd := exec.Command(dockerBinary, cmdArgs...)
	stdout, err := eventsCmd.StdoutPipe()
	if err != nil {
		return nil, err
	}

	return &eventObserver{
		buffer:  new(bytes.Buffer),
		command: eventsCmd,
		stdout:  stdout,
	}, nil
}

// Start starts the events command.
func (e *eventObserver) Start() error {
	return e.command.Start()
}

// Stop stops the events command.
func (e *eventObserver) Stop() {
	e.command.Process.Kill()
}

// Match tries to match the events output with a given matcher.
func (e *eventObserver) Match(match eventMatcher) {
	scanner := bufio.NewScanner(e.stdout)

	for scanner.Scan() {
		text := scanner.Text()
		e.buffer.WriteString(text)
		e.buffer.WriteString("\n")

		match(text)
	}
}

// TimeoutError generates an error for a given containerID and event type.
// It attaches the events command output to the error.
func (e *eventObserver) TimeoutError(id, event string) error {
	return fmt.Errorf("failed to observe event `%s` for %s\n%v", event, id, e.output())
}

// output returns the events command output read until now by the Match goroutine.
func (e *eventObserver) output() string {
	return e.buffer.String()
}
