package process

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"testing"
)

func TestAlive(t *testing.T) {
	for _, pid := range []int{0, -1, -123} {
		t.Run(fmt.Sprintf("invalid process (%d)", pid), func(t *testing.T) {
			if Alive(pid) {
				t.Errorf("PID %d should not be alive", pid)
			}
		})
	}
	t.Run("current process", func(t *testing.T) {
		if pid := os.Getpid(); !Alive(pid) {
			t.Errorf("current PID (%d) should be alive", pid)
		}
	})
	t.Run("exited process", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("TODO: make this work on Windows")
		}

		// Get a PID of an exited process.
		cmd := exec.Command("echo", "hello world")
		err := cmd.Run()
		if err != nil {
			t.Fatal(err)
		}
		exitedPID := cmd.ProcessState.Pid()
		if Alive(exitedPID) {
			t.Errorf("PID %d should not be alive", exitedPID)
		}
	})
}
