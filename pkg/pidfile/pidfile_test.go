package pidfile // import "github.com/docker/docker/pkg/pidfile"

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
)

func TestWrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "testfile")

	err := Write(path, 0)
	if err == nil {
		t.Fatal("writing PID < 1 should fail")
	}

	err = Write(path, os.Getpid())
	if err != nil {
		t.Fatal("Could not create test file", err)
	}

	err = Write(path, os.Getpid())
	if err == nil {
		t.Error("Test file creation not blocked")
	}

	pid, err := Read(path)
	if err != nil {
		t.Error(err)
	}
	if pid != os.Getpid() {
		t.Errorf("expected pid %d, got %d", os.Getpid(), pid)
	}
}

func TestRead(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("non-existing pidFile", func(t *testing.T) {
		_, err := Read(filepath.Join(tmpDir, "nosuchfile"))
		if !errors.Is(err, os.ErrNotExist) {
			t.Errorf("expected an os.ErrNotExist, got: %+v", err)
		}
	})

	// Verify that we ignore a malformed PID in the file.
	t.Run("malformed pid", func(t *testing.T) {
		// Not using Write here, to test Read in isolation.
		pidFile := filepath.Join(tmpDir, "pidfile-malformed")
		err := os.WriteFile(pidFile, []byte("something that's not an integer"), 0o644)
		if err != nil {
			t.Fatal(err)
		}
		pid, err := Read(pidFile)
		if err != nil {
			t.Error(err)
		}
		if pid != 0 {
			t.Errorf("expected pid %d, got %d", 0, pid)
		}
	})

	t.Run("zero pid", func(t *testing.T) {
		// Not using Write here, to test Read in isolation.
		pidFile := filepath.Join(tmpDir, "pidfile-zero")
		err := os.WriteFile(pidFile, []byte(strconv.Itoa(0)), 0o644)
		if err != nil {
			t.Fatal(err)
		}
		pid, err := Read(pidFile)
		if err != nil {
			t.Error(err)
		}
		if pid != 0 {
			t.Errorf("expected pid %d, got %d", 0, pid)
		}
	})

	t.Run("negative pid", func(t *testing.T) {
		// Not using Write here, to test Read in isolation.
		pidFile := filepath.Join(tmpDir, "pidfile-negative")
		err := os.WriteFile(pidFile, []byte(strconv.Itoa(-1)), 0o644)
		if err != nil {
			t.Fatal(err)
		}
		pid, err := Read(pidFile)
		if err != nil {
			t.Error(err)
		}
		if pid != 0 {
			t.Errorf("expected pid %d, got %d", 0, pid)
		}
	})

	t.Run("current process pid", func(t *testing.T) {
		// Not using Write here, to test Read in isolation.
		pidFile := filepath.Join(tmpDir, "pidfile")
		err := os.WriteFile(pidFile, []byte(strconv.Itoa(os.Getpid())), 0o644)
		if err != nil {
			t.Fatal(err)
		}
		pid, err := Read(pidFile)
		if err != nil {
			t.Error(err)
		}
		if pid != os.Getpid() {
			t.Errorf("expected pid %d, got %d", os.Getpid(), pid)
		}
	})

	// Verify that we don't return a PID if the process exited.
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

		// Not using Write here, to test Read in isolation.
		pidFile := filepath.Join(tmpDir, "pidfile-exited")
		err = os.WriteFile(pidFile, []byte(strconv.Itoa(exitedPID)), 0o644)
		if err != nil {
			t.Fatal(err)
		}
		pid, err := Read(pidFile)
		if err != nil {
			t.Error(err)
		}
		if pid != 0 {
			t.Errorf("expected pid %d, got %d", 0, pid)
		}
	})
}
