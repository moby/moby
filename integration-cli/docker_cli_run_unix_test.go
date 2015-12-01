// +build !windows

package main

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/docker/docker/pkg/integration/checker"
	"github.com/docker/docker/pkg/mount"
	"github.com/docker/docker/pkg/parsers"
	"github.com/docker/docker/pkg/sysinfo"
	"github.com/go-check/check"
	"github.com/kr/pty"
)

// #6509
func (s *DockerSuite) TestRunRedirectStdout(c *check.C) {
	checkRedirect := func(command string) {
		_, tty, err := pty.Open()
		c.Assert(err, checker.IsNil, check.Commentf("Could not open pty"))
		cmd := exec.Command("sh", "-c", command)
		cmd.Stdin = tty
		cmd.Stdout = tty
		cmd.Stderr = tty
		c.Assert(cmd.Start(), checker.IsNil)
		ch := make(chan error)
		go func() {
			ch <- cmd.Wait()
			close(ch)
		}()

		select {
		case <-time.After(10 * time.Second):
			c.Fatal("command timeout")
		case err := <-ch:
			c.Assert(err, checker.IsNil, check.Commentf("wait err"))
		}
	}

	checkRedirect(dockerBinary + " run -i busybox cat /etc/passwd | grep -q root")
	checkRedirect(dockerBinary + " run busybox cat /etc/passwd | grep -q root")
}

// Test recursive bind mount works by default
func (s *DockerSuite) TestRunWithVolumesIsRecursive(c *check.C) {
	// /tmp gets permission denied
	testRequires(c, NotUserNamespace)
	tmpDir, err := ioutil.TempDir("", "docker_recursive_mount_test")
	c.Assert(err, checker.IsNil)

	defer os.RemoveAll(tmpDir)

	// Create a temporary tmpfs mount.
	tmpfsDir := filepath.Join(tmpDir, "tmpfs")
	c.Assert(os.MkdirAll(tmpfsDir, 0777), checker.IsNil, check.Commentf("failed to mkdir at %s", tmpfsDir))
	c.Assert(mount.Mount("tmpfs", tmpfsDir, "tmpfs", ""), checker.IsNil, check.Commentf("failed to create a tmpfs mount at %s", tmpfsDir))

	f, err := ioutil.TempFile(tmpfsDir, "touch-me")
	c.Assert(err, checker.IsNil)
	defer f.Close()

	runCmd := exec.Command(dockerBinary, "run", "--name", "test-data", "--volume", fmt.Sprintf("%s:/tmp:ro", tmpDir), "busybox:latest", "ls", "/tmp/tmpfs")
	out, _, _, err := runCommandWithStdoutStderr(runCmd)
	c.Assert(err, checker.IsNil)
	c.Assert(out, checker.Contains, filepath.Base(f.Name()), check.Commentf("Recursive bind mount test failed. Expected file not found"))
}

func (s *DockerSuite) TestRunDeviceDirectory(c *check.C) {
	testRequires(c, DaemonIsLinux, NotUserNamespace)
	if _, err := os.Stat("/dev/snd"); err != nil {
		c.Skip("Host does not have /dev/snd")
	}

	out, _ := dockerCmd(c, "run", "--device", "/dev/snd:/dev/snd", "busybox", "sh", "-c", "ls /dev/snd/")
	c.Assert(strings.Trim(out, "\r\n"), checker.Contains, "timer", check.Commentf("expected output /dev/snd/timer"))

	out, _ = dockerCmd(c, "run", "--device", "/dev/snd:/dev/othersnd", "busybox", "sh", "-c", "ls /dev/othersnd/")
	c.Assert(strings.Trim(out, "\r\n"), checker.Contains, "seq", check.Commentf("expected output /dev/othersnd/seq"))
}

// TestRunDetach checks attaching and detaching with the escape sequence.
func (s *DockerSuite) TestRunAttachDetach(c *check.C) {
	name := "attach-detach"
	cmd := exec.Command(dockerBinary, "run", "--name", name, "-it", "busybox", "cat")
	stdout, err := cmd.StdoutPipe()
	c.Assert(err, checker.IsNil)
	cpty, tty, err := pty.Open()
	c.Assert(err, checker.IsNil)
	defer cpty.Close()
	cmd.Stdin = tty
	c.Assert(cmd.Start(), checker.IsNil)
	c.Assert(waitRun(name), check.IsNil)

	_, err = cpty.Write([]byte("hello\n"))
	c.Assert(err, checker.IsNil)

	out, err := bufio.NewReader(stdout).ReadString('\n')
	c.Assert(err, checker.IsNil)
	c.Assert(strings.TrimSpace(out), checker.Equals, "hello")

	// escape sequence
	_, err = cpty.Write([]byte{16})
	c.Assert(err, checker.IsNil)
	time.Sleep(100 * time.Millisecond)
	_, err = cpty.Write([]byte{17})
	c.Assert(err, checker.IsNil)

	ch := make(chan struct{})
	go func() {
		cmd.Wait()
		ch <- struct{}{}
	}()

	running, err := inspectField(name, "State.Running")
	c.Assert(err, checker.IsNil)
	c.Assert(running, checker.Equals, "true", check.Commentf("expected container to still be running"))

	go func() {
		exec.Command(dockerBinary, "kill", name).Run()
	}()

	select {
	case <-ch:
	case <-time.After(10 * time.Millisecond):
		c.Fatal("timed out waiting for container to exit")
	}
}

func (s *DockerSuite) TestRunWithCPUQuota(c *check.C) {
	testRequires(c, cpuCfsQuota)

	file := "/sys/fs/cgroup/cpu/cpu.cfs_quota_us"
	out, _ := dockerCmd(c, "run", "--cpu-quota", "8000", "--name", "test", "busybox", "cat", file)
	c.Assert(strings.TrimSpace(out), checker.Equals, "8000")

	out, err := inspectField("test", "HostConfig.CpuQuota")
	c.Assert(err, check.IsNil)
	c.Assert(out, checker.Equals, "8000", check.Commentf("setting the CPU CFS quota failed"))
}

func (s *DockerSuite) TestRunWithCpuPeriod(c *check.C) {
	testRequires(c, cpuCfsPeriod)

	file := "/sys/fs/cgroup/cpu/cpu.cfs_period_us"
	out, _ := dockerCmd(c, "run", "--cpu-period", "50000", "--name", "test", "busybox", "cat", file)
	c.Assert(strings.TrimSpace(out), checker.Equals, "50000")

	out, err := inspectField("test", "HostConfig.CpuPeriod")
	c.Assert(err, check.IsNil)
	c.Assert(out, checker.Equals, "50000", check.Commentf("setting the CPU CFS period failed"))
}

func (s *DockerSuite) TestRunWithKernelMemory(c *check.C) {
	testRequires(c, kernelMemorySupport)

	file := "/sys/fs/cgroup/memory/memory.kmem.limit_in_bytes"
	stdout, _, _ := dockerCmdWithStdoutStderr(c, "run", "--kernel-memory", "50M", "--name", "test1", "busybox", "cat", file)
	c.Assert(strings.TrimSpace(stdout), checker.Equals, "52428800")

	out, err := inspectField("test1", "HostConfig.KernelMemory")
	c.Assert(err, check.IsNil)
	c.Assert(out, check.Equals, "52428800")

	out, _, err = dockerCmdWithError("run", "--kernel-memory", "-16m", "--name", "test2", "busybox", "echo", "test")
	expected := "invalid size"
	c.Assert(err, check.NotNil)
	c.Assert(out, checker.Contains, expected)
}

func (s *DockerSuite) TestRunWithCPUShares(c *check.C) {
	testRequires(c, cpuShare)

	file := "/sys/fs/cgroup/cpu/cpu.shares"
	out, _ := dockerCmd(c, "run", "--cpu-shares", "1000", "--name", "test", "busybox", "cat", file)
	c.Assert(strings.TrimSpace(out), checker.Equals, "1000")

	out, err := inspectField("test", "HostConfig.CPUShares")
	c.Assert(err, check.IsNil)
	c.Assert(out, check.Equals, "1000")
}

// "test" should be printed
func (s *DockerSuite) TestRunEchoStdoutWithCPUSharesAndMemoryLimit(c *check.C) {
	testRequires(c, cpuShare)
	testRequires(c, memoryLimitSupport)
	out, _, _ := dockerCmdWithStdoutStderr(c, "run", "--cpu-shares", "1000", "-m", "32m", "busybox", "echo", "test")
	c.Assert(out, checker.Equals, "test\n", check.Commentf("container should've printed 'test'"))
}

func (s *DockerSuite) TestRunWithCpusetCpus(c *check.C) {
	testRequires(c, cgroupCpuset)

	file := "/sys/fs/cgroup/cpuset/cpuset.cpus"
	out, _ := dockerCmd(c, "run", "--cpuset-cpus", "0", "--name", "test", "busybox", "cat", file)
	c.Assert(strings.TrimSpace(out), checker.Equals, "0")

	out, err := inspectField("test", "HostConfig.CpusetCpus")
	c.Assert(err, check.IsNil)
	c.Assert(out, check.Equals, "0")
}

func (s *DockerSuite) TestRunWithCpusetMems(c *check.C) {
	testRequires(c, cgroupCpuset)

	file := "/sys/fs/cgroup/cpuset/cpuset.mems"
	out, _ := dockerCmd(c, "run", "--cpuset-mems", "0", "--name", "test", "busybox", "cat", file)
	c.Assert(strings.TrimSpace(out), checker.Equals, "0")

	out, err := inspectField("test", "HostConfig.CpusetMems")
	c.Assert(err, check.IsNil)
	c.Assert(out, check.Equals, "0")
}

func (s *DockerSuite) TestRunWithBlkioWeight(c *check.C) {
	testRequires(c, blkioWeight)

	file := "/sys/fs/cgroup/blkio/blkio.weight"
	out, _ := dockerCmd(c, "run", "--blkio-weight", "300", "--name", "test", "busybox", "cat", file)
	c.Assert(strings.TrimSpace(out), checker.Equals, "300")

	out, err := inspectField("test", "HostConfig.BlkioWeight")
	c.Assert(err, check.IsNil)
	c.Assert(out, check.Equals, "300")
}

func (s *DockerSuite) TestRunWithBlkioInvalidWeight(c *check.C) {
	testRequires(c, blkioWeight)
	out, _, err := dockerCmdWithError("run", "--blkio-weight", "5", "busybox", "true")
	c.Assert(err, check.NotNil, check.Commentf(out))
	expected := "Range of blkio weight is from 10 to 1000"
	c.Assert(out, checker.Contains, expected)
}

func (s *DockerSuite) TestRunWithBlkioInvalidWeightDevice(c *check.C) {
	testRequires(c, blkioWeight)
	out, _, err := dockerCmdWithError("run", "--blkio-weight-device", "/dev/sda:5", "busybox", "true")
	c.Assert(err, check.NotNil, check.Commentf(out))
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
	case <-time.After(600 * time.Second):
		c.Fatal("Timeout waiting for container to die on OOM")
	}
}

func (s *DockerSuite) TestRunWithMemoryLimit(c *check.C) {
	testRequires(c, memoryLimitSupport)

	file := "/sys/fs/cgroup/memory/memory.limit_in_bytes"
	stdout, _, _ := dockerCmdWithStdoutStderr(c, "run", "-m", "32M", "--name", "test", "busybox", "cat", file)
	c.Assert(strings.TrimSpace(stdout), checker.Equals, "33554432")

	out, err := inspectField("test", "HostConfig.Memory")
	c.Assert(err, check.IsNil)
	c.Assert(out, check.Equals, "33554432")
}

// TestRunWithoutMemoryswapLimit sets memory limit and disables swap
// memory limit, this means the processes in the container can use
// 16M memory and as much swap memory as they need (if the host
// supports swap memory).
func (s *DockerSuite) TestRunWithoutMemoryswapLimit(c *check.C) {
	testRequires(c, DaemonIsLinux)
	testRequires(c, memoryLimitSupport)
	testRequires(c, swapMemorySupport)
	dockerCmd(c, "run", "-m", "32m", "--memory-swap", "-1", "busybox", "true")
}

func (s *DockerSuite) TestRunWithSwappiness(c *check.C) {
	testRequires(c, memorySwappinessSupport)
	file := "/sys/fs/cgroup/memory/memory.swappiness"
	out, _ := dockerCmd(c, "run", "--memory-swappiness", "0", "--name", "test", "busybox", "cat", file)
	c.Assert(strings.TrimSpace(out), checker.Equals, "0")

	out, err := inspectField("test", "HostConfig.MemorySwappiness")
	c.Assert(err, check.IsNil)
	c.Assert(out, check.Equals, "0")
}

func (s *DockerSuite) TestRunWithSwappinessInvalid(c *check.C) {
	testRequires(c, memorySwappinessSupport)
	out, _, err := dockerCmdWithError("run", "--memory-swappiness", "101", "busybox", "true")
	c.Assert(err, check.NotNil)
	expected := "Valid memory swappiness range is 0-100"
	c.Assert(out, checker.Contains, expected, check.Commentf("Expected output to contain %q, not %q", out, expected))

	out, _, err = dockerCmdWithError("run", "--memory-swappiness", "-10", "busybox", "true")
	c.Assert(err, check.NotNil)
	c.Assert(out, checker.Contains, expected, check.Commentf("Expected output to contain %q, not %q", out, expected))
}

func (s *DockerSuite) TestRunWithMemoryReservation(c *check.C) {
	testRequires(c, memoryReservationSupport)

	file := "/sys/fs/cgroup/memory/memory.soft_limit_in_bytes"
	out, _ := dockerCmd(c, "run", "--memory-reservation", "200M", "--name", "test", "busybox", "cat", file)
	c.Assert(strings.TrimSpace(out), checker.Equals, "209715200")

	out, err := inspectField("test", "HostConfig.MemoryReservation")
	c.Assert(err, check.IsNil)
	c.Assert(out, check.Equals, "209715200")
}

func (s *DockerSuite) TestRunWithMemoryReservationInvalid(c *check.C) {
	testRequires(c, memoryLimitSupport)
	testRequires(c, memoryReservationSupport)
	out, _, err := dockerCmdWithError("run", "-m", "500M", "--memory-reservation", "800M", "busybox", "true")
	c.Assert(err, check.NotNil)
	expected := "Minimum memory limit should be larger than memory reservation limit"
	c.Assert(strings.TrimSpace(out), checker.Contains, expected, check.Commentf("run container should fail with invalid memory reservation"))
}

func (s *DockerSuite) TestStopContainerSignal(c *check.C) {
	out, _ := dockerCmd(c, "run", "--stop-signal", "SIGUSR1", "-d", "busybox", "/bin/sh", "-c", `trap 'echo "exit trapped"; exit 0' USR1; while true; do sleep 1; done`)
	containerID := strings.TrimSpace(out)

	c.Assert(waitRun(containerID), checker.IsNil)

	dockerCmd(c, "stop", containerID)
	out, _ = dockerCmd(c, "logs", containerID)

	c.Assert(out, checker.Contains, "exit trapped", check.Commentf("Expected `exit trapped` in the log"))
}

func (s *DockerSuite) TestRunSwapLessThanMemoryLimit(c *check.C) {
	testRequires(c, memoryLimitSupport)
	testRequires(c, swapMemorySupport)
	out, _, err := dockerCmdWithError("run", "-m", "16m", "--memory-swap", "15m", "busybox", "echo", "test")
	expected := "Minimum memoryswap limit should be larger than memory limit"
	c.Assert(err, check.NotNil)

	c.Assert(out, checker.Contains, expected)
}

func (s *DockerSuite) TestRunInvalidCpusetCpusFlagValue(c *check.C) {
	testRequires(c, cgroupCpuset)

	sysInfo := sysinfo.New(true)
	cpus, err := parsers.ParseUintList(sysInfo.Cpus)
	c.Assert(err, check.IsNil)
	var invalid int
	for i := 0; i <= len(cpus)+1; i++ {
		if !cpus[i] {
			invalid = i
			break
		}
	}
	out, _, err := dockerCmdWithError("run", "--cpuset-cpus", strconv.Itoa(invalid), "busybox", "true")
	c.Assert(err, check.NotNil)
	expected := fmt.Sprintf("Error response from daemon: Requested CPUs are not available - requested %s, available: %s", strconv.Itoa(invalid), sysInfo.Cpus)
	c.Assert(out, checker.Contains, expected)
}

func (s *DockerSuite) TestRunInvalidCpusetMemsFlagValue(c *check.C) {
	testRequires(c, cgroupCpuset)

	sysInfo := sysinfo.New(true)
	mems, err := parsers.ParseUintList(sysInfo.Mems)
	c.Assert(err, check.IsNil)
	var invalid int
	for i := 0; i <= len(mems)+1; i++ {
		if !mems[i] {
			invalid = i
			break
		}
	}
	out, _, err := dockerCmdWithError("run", "--cpuset-mems", strconv.Itoa(invalid), "busybox", "true")
	c.Assert(err, check.NotNil)
	expected := fmt.Sprintf("Error response from daemon: Requested memory nodes are not available - requested %s, available: %s", strconv.Itoa(invalid), sysInfo.Mems)
	c.Assert(out, checker.Contains, expected)
}

func (s *DockerSuite) TestRunInvalidCPUShares(c *check.C) {
	testRequires(c, cpuShare, DaemonIsLinux)
	out, _, err := dockerCmdWithError("run", "--cpu-shares", "1", "busybox", "echo", "test")
	c.Assert(err, check.NotNil, check.Commentf(out))
	expected := "The minimum allowed cpu-shares is 2"
	c.Assert(out, checker.Contains, expected)

	out, _, err = dockerCmdWithError("run", "--cpu-shares", "-1", "busybox", "echo", "test")
	c.Assert(err, check.NotNil, check.Commentf(out))
	expected = "shares: invalid argument"
	c.Assert(out, checker.Contains, expected)

	out, _, err = dockerCmdWithError("run", "--cpu-shares", "99999999", "busybox", "echo", "test")
	c.Assert(err, check.NotNil, check.Commentf(out))
	expected = "The maximum allowed cpu-shares is"
	c.Assert(out, checker.Contains, expected)
}

func (s *DockerSuite) TestRunWithDefaultShmSize(c *check.C) {
	testRequires(c, DaemonIsLinux)

	name := "shm-default"
	out, _ := dockerCmd(c, "run", "--name", name, "busybox", "mount")
	shmRegex := regexp.MustCompile(`shm on /dev/shm type tmpfs(.*)size=65536k`)
	if !shmRegex.MatchString(out) {
		c.Fatalf("Expected shm of 64MB in mount command, got %v", out)
	}
	shmSize, err := inspectField(name, "HostConfig.ShmSize")
	c.Assert(err, check.IsNil)
	c.Assert(shmSize, check.Equals, "67108864")
}

func (s *DockerSuite) TestRunWithShmSize(c *check.C) {
	testRequires(c, DaemonIsLinux)

	name := "shm"
	out, _ := dockerCmd(c, "run", "--name", name, "--shm-size=1G", "busybox", "mount")
	shmRegex := regexp.MustCompile(`shm on /dev/shm type tmpfs(.*)size=1048576k`)
	if !shmRegex.MatchString(out) {
		c.Fatalf("Expected shm of 1GB in mount command, got %v", out)
	}
	shmSize, err := inspectField(name, "HostConfig.ShmSize")
	c.Assert(err, check.IsNil)
	c.Assert(shmSize, check.Equals, "1073741824")
}
