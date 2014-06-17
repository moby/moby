package main

import (
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestMultipleAttachRestart(t *testing.T) {
	cmd := exec.Command(dockerBinary, "run", "--name", "attacher", "-d", "busybox",
		"/bin/sh", "-c", "sleep 2 && echo hello")

	group := sync.WaitGroup{}
	group.Add(4)

	defer func() {
		cmd = exec.Command(dockerBinary, "kill", "attacher")
		if _, err := runCommand(cmd); err != nil {
			t.Fatal(err)
		}
		deleteAllContainers()
	}()

	go func() {
		defer group.Done()
		out, _, err := runCommandWithOutput(cmd)
		if err != nil {
			t.Fatal(err, out)
		}
	}()
	time.Sleep(500 * time.Millisecond)

	for i := 0; i < 3; i++ {
		go func() {
			defer group.Done()
			c := exec.Command(dockerBinary, "attach", "attacher")

			out, _, err := runCommandWithOutput(c)
			if err != nil {
				t.Fatal(err, out)
			}
			if actual := strings.Trim(out, "\r\n"); actual != "hello" {
				t.Fatalf("unexpected output %s expected hello", actual)
			}
		}()
	}

	group.Wait()

	logDone("attach - multiple attach")
}
