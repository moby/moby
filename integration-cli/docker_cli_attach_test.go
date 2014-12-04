package main

import (
	"io"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"
)

const attachWait = 5 * time.Second

func TestAttachMultipleAndRestart(t *testing.T) {
	defer deleteAllContainers()

	endGroup := &sync.WaitGroup{}
	startGroup := &sync.WaitGroup{}
	endGroup.Add(3)
	startGroup.Add(3)

	if err := waitForContainer("attacher", "-d", "busybox", "/bin/sh", "-c", "while true; do sleep 1; echo hello; done"); err != nil {
		t.Fatal(err)
	}

	startDone := make(chan struct{})
	endDone := make(chan struct{})

	go func() {
		endGroup.Wait()
		close(endDone)
	}()

	go func() {
		startGroup.Wait()
		close(startDone)
	}()

	for i := 0; i < 3; i++ {
		go func() {
			c := exec.Command(dockerBinary, "attach", "attacher")

			defer func() {
				c.Wait()
				endGroup.Done()
			}()

			out, err := c.StdoutPipe()
			if err != nil {
				t.Fatal(err)
			}

			if err := c.Start(); err != nil {
				t.Fatal(err)
			}

			buf := make([]byte, 1024)

			if _, err := out.Read(buf); err != nil && err != io.EOF {
				t.Fatal(err)
			}

			startGroup.Done()

			if !strings.Contains(string(buf), "hello") {
				t.Fatalf("unexpected output %s expected hello\n", string(buf))
			}
		}()
	}

	select {
	case <-startDone:
	case <-time.After(attachWait):
		t.Fatalf("Attaches did not initialize properly")
	}

	cmd := exec.Command(dockerBinary, "kill", "attacher")
	if _, err := runCommand(cmd); err != nil {
		t.Fatal(err)
	}

	select {
	case <-endDone:
	case <-time.After(attachWait):
		t.Fatalf("Attaches did not finish properly")
	}

	logDone("attach - multiple attach")
}
