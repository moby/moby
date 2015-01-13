// +build !windows

package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/pkg/mount"
	"github.com/kr/pty"
)

// #6509
func TestRunRedirectStdout(t *testing.T) {

	defer deleteAllContainers()

	checkRedirect := func(command string) {
		_, tty, err := pty.Open()
		if err != nil {
			t.Fatalf("Could not open pty: %v", err)
		}
		cmd := exec.Command("sh", "-c", command)
		cmd.Stdin = tty
		cmd.Stdout = tty
		cmd.Stderr = tty
		ch := make(chan struct{})
		if err := cmd.Start(); err != nil {
			t.Fatalf("start err: %v", err)
		}
		go func() {
			if err := cmd.Wait(); err != nil {
				t.Fatalf("wait err=%v", err)
			}
			close(ch)
		}()

		select {
		case <-time.After(10 * time.Second):
			t.Fatal("command timeout")
		case <-ch:
		}
	}

	checkRedirect(dockerBinary + " run -i busybox cat /etc/passwd | grep -q root")
	checkRedirect(dockerBinary + " run busybox cat /etc/passwd | grep -q root")

	logDone("run - redirect stdout")
}

// Test recursive bind mount works by default
func TestRunWithVolumesIsRecursive(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "docker_recursive_mount_test")
	if err != nil {
		t.Fatal(err)
	}

	defer os.RemoveAll(tmpDir)

	// Create a temporary tmpfs mount.
	tmpfsDir := filepath.Join(tmpDir, "tmpfs")
	if err := os.MkdirAll(tmpfsDir, 0777); err != nil {
		t.Fatalf("failed to mkdir at %s - %s", tmpfsDir, err)
	}
	if err := mount.Mount("tmpfs", tmpfsDir, "tmpfs", ""); err != nil {
		t.Fatalf("failed to create a tmpfs mount at %s - %s", tmpfsDir, err)
	}

	f, err := ioutil.TempFile(tmpfsDir, "touch-me")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	runCmd := exec.Command(dockerBinary, "run", "--name", "test-data", "--volume", fmt.Sprintf("%s:/tmp:ro", tmpDir), "busybox:latest", "ls", "/tmp/tmpfs")
	out, stderr, exitCode, err := runCommandWithStdoutStderr(runCmd)
	if err != nil && exitCode != 0 {
		t.Fatal(out, stderr, err)
	}
	if !strings.Contains(out, filepath.Base(f.Name())) {
		t.Fatal("Recursive bind mount test failed. Expected file not found")
	}

	deleteAllContainers()

	logDone("run - volumes are bind mounted recursively")
}
