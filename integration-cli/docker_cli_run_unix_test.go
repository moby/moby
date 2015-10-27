// +build !windows

package main

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
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

func (s *DockerSuite) TestRunDeviceDirectory(c *check.C) {
	testRequires(c, NativeExecDriver)
	if _, err := os.Stat("/dev/snd"); err != nil {
		c.Skip("Host does not have /dev/snd")
	}

	out, _ := dockerCmd(c, "run", "--device", "/dev/snd:/dev/snd", "busybox", "sh", "-c", "ls /dev/snd/")
	if actual := strings.Trim(out, "\r\n"); !strings.Contains(out, "timer") {
		c.Fatalf("expected output /dev/snd/timer, received %s", actual)
	}

	out, _ = dockerCmd(c, "run", "--device", "/dev/snd:/dev/othersnd", "busybox", "sh", "-c", "ls /dev/othersnd/")
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
	c.Assert(waitRun(name), check.IsNil)

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
	testRequires(c, cpuCfsQuota)

	out, _, err := dockerCmdWithError("run", "--cpu-quota", "8000", "--name", "test", "busybox", "echo", "test")
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
	testRequires(c, cpuCfsPeriod)

	if _, _, err := dockerCmdWithError("run", "--cpu-period", "50000", "--name", "test", "busybox", "true"); err != nil {
		c.Fatalf("failed to run container: %v", err)
	}

	out, err := inspectField("test", "HostConfig.CpuPeriod")
	c.Assert(err, check.IsNil)
	if out != "50000" {
		c.Fatalf("setting the CPU CFS period failed")
	}
}

func (s *DockerSuite) TestRunWithKernelMemory(c *check.C) {
	testRequires(c, kernelMemorySupport)

	dockerCmd(c, "run", "--kernel-memory", "50M", "--name", "test", "busybox", "true")

	out, err := inspectField("test", "HostConfig.KernelMemory")
	c.Assert(err, check.IsNil)
	if out != "52428800" {
		c.Fatalf("setting the kernel memory limit failed")
	}
}

// "test" should be printed
func (s *DockerSuite) TestRunEchoStdoutWitCPUShares(c *check.C) {
	testRequires(c, cpuShare)
	out, _ := dockerCmd(c, "run", "--cpu-shares", "1000", "busybox", "echo", "test")
	if out != "test\n" {
		c.Errorf("container should've printed 'test', got %q instead", out)
	}
}

// "test" should be printed
func (s *DockerSuite) TestRunEchoStdoutWithCPUSharesAndMemoryLimit(c *check.C) {
	testRequires(c, cpuShare)
	testRequires(c, memoryLimitSupport)
	out, _, _ := dockerCmdWithStdoutStderr(c, "run", "--cpu-shares", "1000", "-m", "16m", "busybox", "echo", "test")
	if out != "test\n" {
		c.Errorf("container should've printed 'test', got %q instead", out)
	}
}

func (s *DockerSuite) TestRunWithCpuset(c *check.C) {
	testRequires(c, cgroupCpuset)
	if _, code := dockerCmd(c, "run", "--cpuset", "0", "busybox", "true"); code != 0 {
		c.Fatalf("container should run successfully with cpuset of 0")
	}
}

func (s *DockerSuite) TestRunWithCpusetCpus(c *check.C) {
	testRequires(c, cgroupCpuset)
	if _, code := dockerCmd(c, "run", "--cpuset-cpus", "0", "busybox", "true"); code != 0 {
		c.Fatalf("container should run successfully with cpuset-cpus of 0")
	}
}

func (s *DockerSuite) TestRunWithCpusetMems(c *check.C) {
	testRequires(c, cgroupCpuset)
	if _, code := dockerCmd(c, "run", "--cpuset-mems", "0", "busybox", "true"); code != 0 {
		c.Fatalf("container should run successfully with cpuset-mems of 0")
	}
}

func (s *DockerSuite) TestRunWithPidsLimit(c *check.C) {
	testRequires(c, cgroupPids)
	if _, _, code := dockerCmdWithError(c, "run", "--pids-Limit", "0", "busybox", "true|cat"); err == nil {
		c.Fatalf("run with exhausting pids-limit should failed")
	}
}

func (s *DockerSuite) TestRunWithBlkioWeight(c *check.C) {
	testRequires(c, blkioWeight)
	if _, code := dockerCmd(c, "run", "--blkio-weight", "300", "busybox", "true"); code != 0 {
		c.Fatalf("container should run successfully with blkio-weight of 300")
	}
}

func (s *DockerSuite) TestRunWithBlkioInvalidWeight(c *check.C) {
	testRequires(c, blkioWeight)
	if _, _, err := dockerCmdWithError("run", "--blkio-weight", "5", "busybox", "true"); err == nil {
		c.Fatalf("run with invalid blkio-weight should failed")
	}
}

func (s *DockerSuite) TestRunOOMExitCode(c *check.C) {
	testRequires(c, oomControl)
	errChan := make(chan error)
	go func() {
		defer close(errChan)
		//changing memory to 40MB from 4MB due to an issue with GCCGO that test fails to start the container.
		out, exitCode, _ := dockerCmdWithError("run", "-m", "40MB", "busybox", "sh", "-c", "x=a; while true; do x=$x$x$x$x; done")
		if expected := 137; exitCode != expected {
			errChan <- fmt.Errorf("wrong exit code for OOM container: expected %d, got %d (output: %q)", expected, exitCode, out)
		}
	}()

	select {
	case err := <-errChan:
		c.Assert(err, check.IsNil)
	case <-time.After(30 * time.Second):
		c.Fatal("Timeout waiting for container to die on OOM")
	}
}

// "test" should be printed
func (s *DockerSuite) TestRunEchoStdoutWithMemoryLimit(c *check.C) {
	testRequires(c, memoryLimitSupport)
	out, _, _ := dockerCmdWithStdoutStderr(c, "run", "-m", "16m", "busybox", "echo", "test")
	out = strings.Trim(out, "\r\n")

	if expected := "test"; out != expected {
		c.Fatalf("container should've printed %q but printed %q", expected, out)
	}
}

// TestRunWithoutMemoryswapLimit sets memory limit and disables swap
// memory limit, this means the processes in the container can use
// 16M memory and as much swap memory as they need (if the host
// supports swap memory).
func (s *DockerSuite) TestRunWithoutMemoryswapLimit(c *check.C) {
	testRequires(c, NativeExecDriver)
	testRequires(c, memoryLimitSupport)
	testRequires(c, swapMemorySupport)
	dockerCmd(c, "run", "-m", "16m", "--memory-swap", "-1", "busybox", "true")
}

func (s *DockerSuite) TestRunWithSwappiness(c *check.C) {
	testRequires(c, memorySwappinessSupport)
	dockerCmd(c, "run", "--memory-swappiness", "0", "busybox", "true")
}

func (s *DockerSuite) TestRunWithSwappinessInvalid(c *check.C) {
	testRequires(c, memorySwappinessSupport)
	out, _, err := dockerCmdWithError("run", "--memory-swappiness", "101", "busybox", "true")
	if err == nil {
		c.Fatalf("failed. test was able to set invalid value, output: %q", out)
	}
}

func (s *DockerSuite) TestStopContainerSignal(c *check.C) {
	out, _ := dockerCmd(c, "run", "--stop-signal", "SIGUSR1", "-d", "busybox", "/bin/sh", "-c", `trap 'echo "exit trapped"; exit 0' USR1; while true; do sleep 1; done`)
	containerID := strings.TrimSpace(out)

	if err := waitRun(containerID); err != nil {
		c.Fatal(err)
	}

	dockerCmd(c, "stop", containerID)
	out, _ = dockerCmd(c, "logs", containerID)

	if !strings.Contains(out, "exit trapped") {
		c.Fatalf("Expected `exit trapped` in the log, got %v", out)
	}
}
