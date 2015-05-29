// +build !windows

package main

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/docker/docker/pkg/mount"
	"github.com/go-check/check"
	"github.com/kr/pty"
)

// #6509
func (s *DockerSuite) TestRunRedirectStdout(c *check.C) {
	checkRedirect := func(command string) {
		_, tty, err := pty.Open()
		if err != nil {
			c.Fatalf("Could not open pty: %v", err)
		}
		cmd := exec.Command("sh", "-c", command)
		cmd.Stdin = tty
		cmd.Stdout = tty
		cmd.Stderr = tty
		if err := cmd.Start(); err != nil {
			c.Fatalf("start err: %v", err)
		}
		ch := make(chan error)
		go func() {
			ch <- cmd.Wait()
			close(ch)
		}()

		select {
		case <-time.After(10 * time.Second):
			c.Fatal("command timeout")
		case err := <-ch:
			if err != nil {
				c.Fatalf("wait err=%v", err)
			}
		}
	}

	checkRedirect(dockerBinary + " run -i busybox cat /etc/passwd | grep -q root")
	checkRedirect(dockerBinary + " run busybox cat /etc/passwd | grep -q root")
}

// Test recursive bind mount works by default
func (s *DockerSuite) TestRunWithVolumesIsRecursive(c *check.C) {
	tmpDir, err := ioutil.TempDir("", "docker_recursive_mount_test")
	if err != nil {
		c.Fatal(err)
	}

	defer os.RemoveAll(tmpDir)

	// Create a temporary tmpfs mount.
	tmpfsDir := filepath.Join(tmpDir, "tmpfs")
	if err := os.MkdirAll(tmpfsDir, 0777); err != nil {
		c.Fatalf("failed to mkdir at %s - %s", tmpfsDir, err)
	}
	if err := mount.Mount("tmpfs", tmpfsDir, "tmpfs", ""); err != nil {
		c.Fatalf("failed to create a tmpfs mount at %s - %s", tmpfsDir, err)
	}

	f, err := ioutil.TempFile(tmpfsDir, "touch-me")
	if err != nil {
		c.Fatal(err)
	}
	defer f.Close()

	runCmd := exec.Command(dockerBinary, "run", "--name", "test-data", "--volume", fmt.Sprintf("%s:/tmp:ro", tmpDir), "busybox:latest", "ls", "/tmp/tmpfs")
	out, stderr, exitCode, err := runCommandWithStdoutStderr(runCmd)
	if err != nil && exitCode != 0 {
		c.Fatal(out, stderr, err)
	}
	if !strings.Contains(out, filepath.Base(f.Name())) {
		c.Fatal("Recursive bind mount test failed. Expected file not found")
	}
}

func (s *DockerSuite) TestRunWithUlimits(c *check.C) {
	testRequires(c, NativeExecDriver)
	out, _, err := runCommandWithOutput(exec.Command(dockerBinary, "run", "--name=testulimits", "--ulimit", "nofile=42", "busybox", "/bin/sh", "-c", "ulimit -n"))
	if err != nil {
		c.Fatal(err, out)
	}

	ul := strings.TrimSpace(out)
	if ul != "42" {
		c.Fatalf("expected `ulimit -n` to be 42, got %s", ul)
	}
}

func (s *DockerSuite) TestRunContainerWithCgroupParent(c *check.C) {
	testRequires(c, NativeExecDriver)

	cgroupParent := "test"
	data, err := ioutil.ReadFile("/proc/self/cgroup")
	if err != nil {
		c.Fatalf("failed to read '/proc/self/cgroup - %v", err)
	}
	selfCgroupPaths := parseCgroupPaths(string(data))
	selfCpuCgroup, found := selfCgroupPaths["memory"]
	if !found {
		c.Fatalf("unable to find self cpu cgroup path. CgroupsPath: %v", selfCgroupPaths)
	}

	out, _, err := runCommandWithOutput(exec.Command(dockerBinary, "run", "--cgroup-parent", cgroupParent, "--rm", "busybox", "cat", "/proc/self/cgroup"))
	if err != nil {
		c.Fatalf("unexpected failure when running container with --cgroup-parent option - %s\n%v", string(out), err)
	}
	cgroupPaths := parseCgroupPaths(string(out))
	if len(cgroupPaths) == 0 {
		c.Fatalf("unexpected output - %q", string(out))
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
		c.Fatalf("unexpected cgroup paths. Expected at least one cgroup path to have prefix %q. Cgroup Paths: %v", expectedCgroupPrefix, cgroupPaths)
	}
}

func (s *DockerSuite) TestRunContainerWithCgroupParentAbsPath(c *check.C) {
	testRequires(c, NativeExecDriver)

	cgroupParent := "/cgroup-parent/test"

	out, _, err := runCommandWithOutput(exec.Command(dockerBinary, "run", "--cgroup-parent", cgroupParent, "--rm", "busybox", "cat", "/proc/self/cgroup"))
	if err != nil {
		c.Fatalf("unexpected failure when running container with --cgroup-parent option - %s\n%v", string(out), err)
	}
	cgroupPaths := parseCgroupPaths(string(out))
	if len(cgroupPaths) == 0 {
		c.Fatalf("unexpected output - %q", string(out))
	}
	found := false
	for _, path := range cgroupPaths {
		if strings.HasPrefix(path, cgroupParent) {
			found = true
			break
		}
	}
	if !found {
		c.Fatalf("unexpected cgroup paths. Expected at least one cgroup path to have prefix %q. Cgroup Paths: %v", cgroupParent, cgroupPaths)
	}
}

func (s *DockerSuite) TestRunDeviceDirectory(c *check.C) {
	testRequires(c, NativeExecDriver)
	cmd := exec.Command(dockerBinary, "run", "--device", "/dev/snd:/dev/snd", "busybox", "sh", "-c", "ls /dev/snd/")

	out, _, err := runCommandWithOutput(cmd)
	if err != nil {
		c.Fatal(err, out)
	}

	if actual := strings.Trim(out, "\r\n"); !strings.Contains(out, "timer") {
		c.Fatalf("expected output /dev/snd/timer, received %s", actual)
	}

	cmd = exec.Command(dockerBinary, "run", "--device", "/dev/snd:/dev/othersnd", "busybox", "sh", "-c", "ls /dev/othersnd/")

	out, _, err = runCommandWithOutput(cmd)
	if err != nil {
		c.Fatal(err, out)
	}

	if actual := strings.Trim(out, "\r\n"); !strings.Contains(out, "seq") {
		c.Fatalf("expected output /dev/othersnd/seq, received %s", actual)
	}
}

// TestRunDetach checks attaching and detaching with the escape sequence.
func (s *DockerSuite) TestRunAttachDetach(c *check.C) {
	name := "attach-detach"
	cmd := exec.Command(dockerBinary, "run", "--name", name, "-it", "busybox", "cat")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		c.Fatal(err)
	}
	cpty, tty, err := pty.Open()
	if err != nil {
		c.Fatal(err)
	}
	defer cpty.Close()
	cmd.Stdin = tty
	if err := cmd.Start(); err != nil {
		c.Fatal(err)
	}
	if err := waitRun(name); err != nil {
		c.Fatal(err)
	}

	if _, err := cpty.Write([]byte("hello\n")); err != nil {
		c.Fatal(err)
	}

	out, err := bufio.NewReader(stdout).ReadString('\n')
	if err != nil {
		c.Fatal(err)
	}
	if strings.TrimSpace(out) != "hello" {
		c.Fatalf("expected 'hello', got %q", out)
	}

	// escape sequence
	if _, err := cpty.Write([]byte{16}); err != nil {
		c.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)
	if _, err := cpty.Write([]byte{17}); err != nil {
		c.Fatal(err)
	}

	ch := make(chan struct{})
	go func() {
		cmd.Wait()
		ch <- struct{}{}
	}()

	running, err := inspectField(name, "State.Running")
	if err != nil {
		c.Fatal(err)
	}
	if running != "true" {
		c.Fatal("expected container to still be running")
	}

	go func() {
		exec.Command(dockerBinary, "kill", name).Run()
	}()

	select {
	case <-ch:
	case <-time.After(10 * time.Millisecond):
		c.Fatal("timed out waiting for container to exit")
	}
}

// "test" should be printed
func (s *DockerSuite) TestRunEchoStdoutWithCPUQuota(c *check.C) {
	testRequires(c, CpuCfsQuota)
	runCmd := exec.Command(dockerBinary, "run", "--cpu-quota", "8000", "--name", "test", "busybox", "echo", "test")
	out, _, _, err := runCommandWithStdoutStderr(runCmd)
	if err != nil {
		c.Fatalf("failed to run container: %v, output: %q", err, out)
	}
	out = strings.TrimSpace(out)
	if out != "test" {
		c.Errorf("container should've printed 'test'")
	}

	out, err = inspectField("test", "HostConfig.CpuQuota")
	c.Assert(err, check.IsNil)

	if out != "8000" {
		c.Fatalf("setting the CPU CFS quota failed")
	}
}

func (s *DockerSuite) TestRunWithCpuPeriod(c *check.C) {
	testRequires(c, CpuCfsPeriod)
	runCmd := exec.Command(dockerBinary, "run", "--cpu-period", "50000", "--name", "test", "busybox", "true")
	if _, err := runCommand(runCmd); err != nil {
		c.Fatalf("failed to run container: %v", err)
	}

	out, err := inspectField("test", "HostConfig.CpuPeriod")
	c.Assert(err, check.IsNil)
	if out != "50000" {
		c.Fatalf("setting the CPU CFS period failed")
	}
}
