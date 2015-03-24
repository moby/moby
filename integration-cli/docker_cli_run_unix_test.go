// +build !windows

package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
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
	defer deleteAllContainers()

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

	logDone("run - volumes are bind mounted recursively")
}

func TestRunWithUlimits(t *testing.T) {
	testRequires(t, NativeExecDriver)
	defer deleteAllContainers()
	out, _, err := runCommandWithOutput(exec.Command(dockerBinary, "run", "--name=testulimits", "--ulimit", "nofile=42", "busybox", "/bin/sh", "-c", "ulimit -n"))
	if err != nil {
		t.Fatal(err, out)
	}

	ul := strings.TrimSpace(out)
	if ul != "42" {
		t.Fatalf("expected `ulimit -n` to be 42, got %s", ul)
	}

	logDone("run - ulimits are set")
}

func TestRunContainerWithCgroupParent(t *testing.T) {
	testRequires(t, NativeExecDriver)
	defer deleteAllContainers()

	cgroupParent := "test"
	data, err := ioutil.ReadFile("/proc/self/cgroup")
	if err != nil {
		t.Fatalf("failed to read '/proc/self/cgroup - %v", err)
	}
	selfCgroupPaths := parseCgroupPaths(string(data))
	selfCpuCgroup, found := selfCgroupPaths["memory"]
	if !found {
		t.Fatalf("unable to find self cpu cgroup path. CgroupsPath: %v", selfCgroupPaths)
	}

	out, _, err := runCommandWithOutput(exec.Command(dockerBinary, "run", "--cgroup-parent", cgroupParent, "--rm", "busybox", "cat", "/proc/self/cgroup"))
	if err != nil {
		t.Fatalf("unexpected failure when running container with --cgroup-parent option - %s\n%v", string(out), err)
	}
	cgroupPaths := parseCgroupPaths(string(out))
	if len(cgroupPaths) == 0 {
		t.Fatalf("unexpected output - %q", string(out))
	}
	found = false
	expectedCgroupPrefix := path.Join(selfCpuCgroup, cgroupParent)
	for _, path := range cgroupPaths {
		if strings.HasPrefix(path, expectedCgroupPrefix) {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("unexpected cgroup paths. Expected at least one cgroup path to have prefix %q. Cgroup Paths: %v", expectedCgroupPrefix, cgroupPaths)
	}
	logDone("run - cgroup parent")
}

func TestRunContainerWithCgroupParentAbsPath(t *testing.T) {
	testRequires(t, NativeExecDriver)
	defer deleteAllContainers()

	cgroupParent := "/cgroup-parent/test"

	out, _, err := runCommandWithOutput(exec.Command(dockerBinary, "run", "--cgroup-parent", cgroupParent, "--rm", "busybox", "cat", "/proc/self/cgroup"))
	if err != nil {
		t.Fatalf("unexpected failure when running container with --cgroup-parent option - %s\n%v", string(out), err)
	}
	cgroupPaths := parseCgroupPaths(string(out))
	if len(cgroupPaths) == 0 {
		t.Fatalf("unexpected output - %q", string(out))
	}
	found := false
	for _, path := range cgroupPaths {
		if strings.HasPrefix(path, cgroupParent) {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("unexpected cgroup paths. Expected at least one cgroup path to have prefix %q. Cgroup Paths: %v", cgroupParent, cgroupPaths)
	}

	logDone("run - cgroup parent with absolute cgroup path")
}
