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
	"sync"
	"time"

	"github.com/docker/docker/pkg/homedir"
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
	testRequires(c, DaemonIsLinux, NotUserNamespace, NotArm)
	if _, err := os.Stat("/dev/snd"); err != nil {
		c.Skip("Host does not have /dev/snd")
	}

	out, _ := dockerCmd(c, "run", "--device", "/dev/snd:/dev/snd", "busybox", "sh", "-c", "ls /dev/snd/")
	c.Assert(strings.Trim(out, "\r\n"), checker.Contains, "timer", check.Commentf("expected output /dev/snd/timer"))

	out, _ = dockerCmd(c, "run", "--device", "/dev/snd:/dev/othersnd", "busybox", "sh", "-c", "ls /dev/othersnd/")
	c.Assert(strings.Trim(out, "\r\n"), checker.Contains, "seq", check.Commentf("expected output /dev/othersnd/seq"))
}

// TestRunDetach checks attaching and detaching with the default escape sequence.
func (s *DockerSuite) TestRunAttachDetach(c *check.C) {
	name := "attach-detach"

	dockerCmd(c, "run", "--name", name, "-itd", "busybox", "cat")

	cmd := exec.Command(dockerBinary, "attach", name)
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

	select {
	case <-ch:
	case <-time.After(10 * time.Second):
		c.Fatal("timed out waiting for container to exit")
	}

	running, err := inspectField(name, "State.Running")
	c.Assert(err, checker.IsNil)
	c.Assert(running, checker.Equals, "true", check.Commentf("expected container to still be running"))
}

// TestRunDetach checks attaching and detaching with the escape sequence specified via flags.
func (s *DockerSuite) TestRunAttachDetachFromFlag(c *check.C) {
	name := "attach-detach"
	keyCtrlA := []byte{1}
	keyA := []byte{97}

	dockerCmd(c, "run", "--name", name, "-itd", "busybox", "cat")

	cmd := exec.Command(dockerBinary, "attach", "--detach-keys='ctrl-a,a'", name)
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
	if _, err := cpty.Write(keyCtrlA); err != nil {
		c.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)
	if _, err := cpty.Write(keyA); err != nil {
		c.Fatal(err)
	}

	ch := make(chan struct{})
	go func() {
		cmd.Wait()
		ch <- struct{}{}
	}()

	select {
	case <-ch:
	case <-time.After(10 * time.Second):
		c.Fatal("timed out waiting for container to exit")
	}

	running, err := inspectField(name, "State.Running")
	c.Assert(err, checker.IsNil)
	c.Assert(running, checker.Equals, "true", check.Commentf("expected container to still be running"))
}

// TestRunDetach checks attaching and detaching with the escape sequence specified via config file.
func (s *DockerSuite) TestRunAttachDetachFromConfig(c *check.C) {
	keyCtrlA := []byte{1}
	keyA := []byte{97}

	// Setup config
	homeKey := homedir.Key()
	homeVal := homedir.Get()
	tmpDir, err := ioutil.TempDir("", "fake-home")
	c.Assert(err, checker.IsNil)
	defer os.RemoveAll(tmpDir)

	dotDocker := filepath.Join(tmpDir, ".docker")
	os.Mkdir(dotDocker, 0600)
	tmpCfg := filepath.Join(dotDocker, "config.json")

	defer func() { os.Setenv(homeKey, homeVal) }()
	os.Setenv(homeKey, tmpDir)

	data := `{
		"detachKeys": "ctrl-a,a"
	}`

	err = ioutil.WriteFile(tmpCfg, []byte(data), 0600)
	c.Assert(err, checker.IsNil)

	// Then do the work
	name := "attach-detach"
	dockerCmd(c, "run", "--name", name, "-itd", "busybox", "cat")

	cmd := exec.Command(dockerBinary, "attach", name)
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
	if _, err := cpty.Write(keyCtrlA); err != nil {
		c.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)
	if _, err := cpty.Write(keyA); err != nil {
		c.Fatal(err)
	}

	ch := make(chan struct{})
	go func() {
		cmd.Wait()
		ch <- struct{}{}
	}()

	select {
	case <-ch:
	case <-time.After(10 * time.Second):
		c.Fatal("timed out waiting for container to exit")
	}

	running, err := inspectField(name, "State.Running")
	c.Assert(err, checker.IsNil)
	c.Assert(running, checker.Equals, "true", check.Commentf("expected container to still be running"))
}

// TestRunDetach checks attaching and detaching with the detach flags, making sure it overrides config file
func (s *DockerSuite) TestRunAttachDetachKeysOverrideConfig(c *check.C) {
	keyCtrlA := []byte{1}
	keyA := []byte{97}

	// Setup config
	homeKey := homedir.Key()
	homeVal := homedir.Get()
	tmpDir, err := ioutil.TempDir("", "fake-home")
	c.Assert(err, checker.IsNil)
	defer os.RemoveAll(tmpDir)

	dotDocker := filepath.Join(tmpDir, ".docker")
	os.Mkdir(dotDocker, 0600)
	tmpCfg := filepath.Join(dotDocker, "config.json")

	defer func() { os.Setenv(homeKey, homeVal) }()
	os.Setenv(homeKey, tmpDir)

	data := `{
		"detachKeys": "ctrl-e,e"
	}`

	err = ioutil.WriteFile(tmpCfg, []byte(data), 0600)
	c.Assert(err, checker.IsNil)

	// Then do the work
	name := "attach-detach"
	dockerCmd(c, "run", "--name", name, "-itd", "busybox", "cat")

	cmd := exec.Command(dockerBinary, "attach", "--detach-keys='ctrl-a,a'", name)
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
	if _, err := cpty.Write(keyCtrlA); err != nil {
		c.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)
	if _, err := cpty.Write(keyA); err != nil {
		c.Fatal(err)
	}

	ch := make(chan struct{})
	go func() {
		cmd.Wait()
		ch <- struct{}{}
	}()

	select {
	case <-ch:
	case <-time.After(10 * time.Second):
		c.Fatal("timed out waiting for container to exit")
	}

	running, err := inspectField(name, "State.Running")
	c.Assert(err, checker.IsNil)
	c.Assert(running, checker.Equals, "true", check.Commentf("expected container to still be running"))
}

// "test" should be printed
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
}

func (s *DockerSuite) TestRunWithInvalidKernelMemory(c *check.C) {
	testRequires(c, kernelMemorySupport)

	out, _, err := dockerCmdWithError("run", "--kernel-memory", "2M", "busybox", "true")
	c.Assert(err, check.NotNil)
	expected := "Minimum kernel memory limit allowed is 4MB"
	c.Assert(out, checker.Contains, expected)

	out, _, err = dockerCmdWithError("run", "--kernel-memory", "-16m", "--name", "test2", "busybox", "echo", "test")
	c.Assert(err, check.NotNil)
	expected = "invalid size"
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

func (s *DockerSuite) TestRunWithInvalidBlkioWeight(c *check.C) {
	testRequires(c, blkioWeight)
	out, _, err := dockerCmdWithError("run", "--blkio-weight", "5", "busybox", "true")
	c.Assert(err, check.NotNil, check.Commentf(out))
	expected := "Range of blkio weight is from 10 to 1000"
	c.Assert(out, checker.Contains, expected)
}

func (s *DockerSuite) TestRunWithInvalidPathforBlkioWeightDevice(c *check.C) {
	testRequires(c, blkioWeight)
	out, _, err := dockerCmdWithError("run", "--blkio-weight-device", "/dev/sdX:100", "busybox", "true")
	c.Assert(err, check.NotNil, check.Commentf(out))
}

func (s *DockerSuite) TestRunWithInvalidPathforBlkioDeviceReadBps(c *check.C) {
	testRequires(c, blkioWeight)
	out, _, err := dockerCmdWithError("run", "--device-read-bps", "/dev/sdX:500", "busybox", "true")
	c.Assert(err, check.NotNil, check.Commentf(out))
}

func (s *DockerSuite) TestRunWithInvalidPathforBlkioDeviceWriteBps(c *check.C) {
	testRequires(c, blkioWeight)
	out, _, err := dockerCmdWithError("run", "--device-write-bps", "/dev/sdX:500", "busybox", "true")
	c.Assert(err, check.NotNil, check.Commentf(out))
}

func (s *DockerSuite) TestRunWithInvalidPathforBlkioDeviceReadIOps(c *check.C) {
	testRequires(c, blkioWeight)
	out, _, err := dockerCmdWithError("run", "--device-read-iops", "/dev/sdX:500", "busybox", "true")
	c.Assert(err, check.NotNil, check.Commentf(out))
}

func (s *DockerSuite) TestRunWithInvalidPathforBlkioDeviceWriteIOps(c *check.C) {
	testRequires(c, blkioWeight)
	out, _, err := dockerCmdWithError("run", "--device-write-iops", "/dev/sdX:500", "busybox", "true")
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

func (s *DockerSuite) TestRunTmpfsMounts(c *check.C) {
	// TODO Windows (Post TP4): This test cannot run on a Windows daemon as
	// Windows does not support tmpfs mounts.
	testRequires(c, DaemonIsLinux)
	if out, _, err := dockerCmdWithError("run", "--tmpfs", "/run", "busybox", "touch", "/run/somefile"); err != nil {
		c.Fatalf("/run directory not mounted on tmpfs %q %s", err, out)
	}
	if out, _, err := dockerCmdWithError("run", "--tmpfs", "/run:noexec", "busybox", "touch", "/run/somefile"); err != nil {
		c.Fatalf("/run directory not mounted on tmpfs %q %s", err, out)
	}
	if out, _, err := dockerCmdWithError("run", "--tmpfs", "/run:noexec,nosuid,rw,size=5k,mode=700", "busybox", "touch", "/run/somefile"); err != nil {
		c.Fatalf("/run failed to mount on tmpfs with valid options %q %s", err, out)
	}
	if _, _, err := dockerCmdWithError("run", "--tmpfs", "/run:foobar", "busybox", "touch", "/run/somefile"); err == nil {
		c.Fatalf("/run mounted on tmpfs when it should have vailed within invalid mount option")
	}
	if _, _, err := dockerCmdWithError("run", "--tmpfs", "/run", "-v", "/run:/run", "busybox", "touch", "/run/somefile"); err == nil {
		c.Fatalf("Should have generated an error saying Duplicate mount  points")
	}
}

// TestRunSeccompProfileDenyUnshare checks that 'docker run --security-opt seccomp:/tmp/profile.json debian:jessie unshare' exits with operation not permitted.
func (s *DockerSuite) TestRunSeccompProfileDenyUnshare(c *check.C) {
	testRequires(c, SameHostDaemon, seccompEnabled, NotArm, Apparmor)
	jsonData := `{
	"defaultAction": "SCMP_ACT_ALLOW",
	"syscalls": [
		{
			"name": "unshare",
			"action": "SCMP_ACT_ERRNO"
		}
	]
}`
	tmpFile, err := ioutil.TempFile("", "profile.json")
	defer tmpFile.Close()
	if err != nil {
		c.Fatal(err)
	}

	if _, err := tmpFile.Write([]byte(jsonData)); err != nil {
		c.Fatal(err)
	}
	runCmd := exec.Command(dockerBinary, "run", "--security-opt", "apparmor:unconfined", "--security-opt", "seccomp:"+tmpFile.Name(), "debian:jessie", "unshare", "-p", "-m", "-f", "-r", "mount", "-t", "proc", "none", "/proc")
	out, _, _ := runCommandWithOutput(runCmd)
	if !strings.Contains(out, "Operation not permitted") {
		c.Fatalf("expected unshare with seccomp profile denied to fail, got %s", out)
	}
}

// TestRunSeccompProfileDenyChmod checks that 'docker run --security-opt seccomp:/tmp/profile.json busybox chmod 400 /etc/hostname' exits with operation not permitted.
func (s *DockerSuite) TestRunSeccompProfileDenyChmod(c *check.C) {
	testRequires(c, SameHostDaemon, seccompEnabled)
	jsonData := `{
	"defaultAction": "SCMP_ACT_ALLOW",
	"syscalls": [
		{
			"name": "chmod",
			"action": "SCMP_ACT_ERRNO"
		}
	]
}`
	tmpFile, err := ioutil.TempFile("", "profile.json")
	defer tmpFile.Close()
	if err != nil {
		c.Fatal(err)
	}

	if _, err := tmpFile.Write([]byte(jsonData)); err != nil {
		c.Fatal(err)
	}
	runCmd := exec.Command(dockerBinary, "run", "--security-opt", "seccomp:"+tmpFile.Name(), "busybox", "chmod", "400", "/etc/hostname")
	out, _, _ := runCommandWithOutput(runCmd)
	if !strings.Contains(out, "Operation not permitted") {
		c.Fatalf("expected chmod with seccomp profile denied to fail, got %s", out)
	}
}

// TestRunSeccompProfileDenyUnshareUserns checks that 'docker run debian:jessie unshare --map-root-user --user sh -c whoami' with a specific profile to
// deny unhare of a userns exits with operation not permitted.
func (s *DockerSuite) TestRunSeccompProfileDenyUnshareUserns(c *check.C) {
	testRequires(c, SameHostDaemon, seccompEnabled, NotArm, Apparmor)
	// from sched.h
	jsonData := fmt.Sprintf(`{
	"defaultAction": "SCMP_ACT_ALLOW",
	"syscalls": [
		{
			"name": "unshare",
			"action": "SCMP_ACT_ERRNO",
			"args": [
				{
					"index": 0,
					"value": %d,
					"op": "SCMP_CMP_EQ"
				}
			]
		}
	]
}`, uint64(0x10000000))
	tmpFile, err := ioutil.TempFile("", "profile.json")
	defer tmpFile.Close()
	if err != nil {
		c.Fatal(err)
	}

	if _, err := tmpFile.Write([]byte(jsonData)); err != nil {
		c.Fatal(err)
	}
	runCmd := exec.Command(dockerBinary, "run", "--security-opt", "apparmor:unconfined", "--security-opt", "seccomp:"+tmpFile.Name(), "debian:jessie", "unshare", "--map-root-user", "--user", "sh", "-c", "whoami")
	out, _, _ := runCommandWithOutput(runCmd)
	if !strings.Contains(out, "Operation not permitted") {
		c.Fatalf("expected unshare userns with seccomp profile denied to fail, got %s", out)
	}
}

// TestRunSeccompProfileDenyCloneUserns checks that 'docker run syscall-test'
// with a the default seccomp profile exits with operation not permitted.
func (s *DockerSuite) TestRunSeccompProfileDenyCloneUserns(c *check.C) {
	testRequires(c, SameHostDaemon, seccompEnabled)

	runCmd := exec.Command(dockerBinary, "run", "syscall-test", "userns-test", "id")
	out, _, err := runCommandWithOutput(runCmd)
	if err == nil || !strings.Contains(out, "clone failed: Operation not permitted") {
		c.Fatalf("expected clone userns with default seccomp profile denied to fail, got %s: %v", out, err)
	}
}

// TestRunSeccompUnconfinedCloneUserns checks that
// 'docker run --security-opt seccomp:unconfined syscall-test' allows creating a userns.
func (s *DockerSuite) TestRunSeccompUnconfinedCloneUserns(c *check.C) {
	testRequires(c, SameHostDaemon, seccompEnabled, NotUserNamespace)

	// make sure running w privileged is ok
	runCmd := exec.Command(dockerBinary, "run", "--security-opt", "seccomp:unconfined", "syscall-test", "userns-test", "id")
	if out, _, err := runCommandWithOutput(runCmd); err != nil || !strings.Contains(out, "nobody") {
		c.Fatalf("expected clone userns with --security-opt seccomp:unconfined to succeed, got %s: %v", out, err)
	}
}

// TestRunSeccompAllowPrivCloneUserns checks that 'docker run --privileged syscall-test'
// allows creating a userns.
func (s *DockerSuite) TestRunSeccompAllowPrivCloneUserns(c *check.C) {
	testRequires(c, SameHostDaemon, seccompEnabled, NotUserNamespace)

	// make sure running w privileged is ok
	runCmd := exec.Command(dockerBinary, "run", "--privileged", "syscall-test", "userns-test", "id")
	if out, _, err := runCommandWithOutput(runCmd); err != nil || !strings.Contains(out, "nobody") {
		c.Fatalf("expected clone userns with --privileged to succeed, got %s: %v", out, err)
	}
}

// TestRunSeccompAllowAptKey checks that 'docker run debian:jessie apt-key' succeeds.
func (s *DockerSuite) TestRunSeccompAllowAptKey(c *check.C) {
	testRequires(c, SameHostDaemon, seccompEnabled, Network)

	// apt-key uses setrlimit & getrlimit, so we want to make sure we don't break it
	runCmd := exec.Command(dockerBinary, "run", "debian:jessie", "apt-key", "adv", "--keyserver", "hkp://p80.pool.sks-keyservers.net:80", "--recv-keys", "E871F18B51E0147C77796AC81196BA81F6B0FC61")
	if out, _, err := runCommandWithOutput(runCmd); err != nil {
		c.Fatalf("expected apt-key with seccomp to succeed, got %s: %v", out, err)
	}
}

func (s *DockerSuite) TestRunSeccompDefaultProfile(c *check.C) {
	testRequires(c, SameHostDaemon, seccompEnabled, NotUserNamespace)

	var group sync.WaitGroup
	group.Add(4)
	errChan := make(chan error, 4)
	go func() {
		out, _, err := dockerCmdWithError("run", "--cap-add", "ALL", "syscall-test", "acct-test")
		if err == nil || !strings.Contains(out, "Operation not permitted") {
			errChan <- fmt.Errorf("expected Operation not permitted, got: %s", out)
		}
		group.Done()
	}()

	go func() {
		out, _, err := dockerCmdWithError("run", "--cap-add", "ALL", "syscall-test", "ns-test", "echo", "hello")
		if err == nil || !strings.Contains(out, "Operation not permitted") {
			errChan <- fmt.Errorf("expected Operation not permitted, got: %s", out)
		}
		group.Done()
	}()

	go func() {
		out, _, err := dockerCmdWithError("run", "--cap-add", "ALL", "--security-opt", "seccomp:unconfined", "syscall-test", "acct-test")
		if err == nil || !strings.Contains(out, "No such file or directory") {
			errChan <- fmt.Errorf("expected No such file or directory, got: %s", out)
		}
		group.Done()
	}()

	go func() {
		out, _, err := dockerCmdWithError("run", "--cap-add", "ALL", "--security-opt", "seccomp:unconfined", "syscall-test", "ns-test", "echo", "hello")
		if err != nil || !strings.Contains(out, "hello") {
			errChan <- fmt.Errorf("expected hello, got: %s, %v", out, err)
		}
		group.Done()
	}()

	group.Wait()
	close(errChan)

	for err := range errChan {
		c.Assert(err, checker.IsNil)
	}
}

func (s *DockerSuite) TestRunApparmorProcDirectory(c *check.C) {
	testRequires(c, SameHostDaemon, Apparmor)

	// running w seccomp unconfined tests the apparmor profile
	runCmd := exec.Command(dockerBinary, "run", "--security-opt", "seccomp:unconfined", "debian:jessie", "chmod", "777", "/proc/1/cgroup")
	if out, _, err := runCommandWithOutput(runCmd); err == nil || !(strings.Contains(out, "Permission denied") || strings.Contains(out, "Operation not permitted")) {
		c.Fatalf("expected chmod 777 /proc/1/cgroup to fail, got %s: %v", out, err)
	}

	runCmd = exec.Command(dockerBinary, "run", "--security-opt", "seccomp:unconfined", "debian:jessie", "chmod", "777", "/proc/1/attr/current")
	if out, _, err := runCommandWithOutput(runCmd); err == nil || !(strings.Contains(out, "Permission denied") || strings.Contains(out, "Operation not permitted")) {
		c.Fatalf("expected chmod 777 /proc/1/attr/current to fail, got %s: %v", out, err)
	}
}
