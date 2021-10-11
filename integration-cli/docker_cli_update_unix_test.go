//go:build !windows
// +build !windows

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/creack/pty"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/docker/docker/testutil/request"
	"gotest.tools/v3/assert"
)

func (s *DockerSuite) TestUpdateRunningContainer(c *testing.T) {
	testRequires(c, DaemonIsLinux)
	testRequires(c, memoryLimitSupport)

	name := "test-update-container"
	dockerCmd(c, "run", "-d", "--name", name, "-m", "300M", "busybox", "top")
	dockerCmd(c, "update", "-m", "500M", name)

	assert.Equal(c, inspectField(c, name, "HostConfig.Memory"), "524288000")

	file := "/sys/fs/cgroup/memory/memory.limit_in_bytes"
	out, _ := dockerCmd(c, "exec", name, "cat", file)
	assert.Equal(c, strings.TrimSpace(out), "524288000")
}

func (s *DockerSuite) TestUpdateRunningContainerWithRestart(c *testing.T) {
	testRequires(c, DaemonIsLinux)
	testRequires(c, memoryLimitSupport)

	name := "test-update-container"
	dockerCmd(c, "run", "-d", "--name", name, "-m", "300M", "busybox", "top")
	dockerCmd(c, "update", "-m", "500M", name)
	dockerCmd(c, "restart", name)

	assert.Equal(c, inspectField(c, name, "HostConfig.Memory"), "524288000")

	file := "/sys/fs/cgroup/memory/memory.limit_in_bytes"
	out, _ := dockerCmd(c, "exec", name, "cat", file)
	assert.Equal(c, strings.TrimSpace(out), "524288000")
}

func (s *DockerSuite) TestUpdateStoppedContainer(c *testing.T) {
	testRequires(c, DaemonIsLinux)
	testRequires(c, memoryLimitSupport)

	name := "test-update-container"
	file := "/sys/fs/cgroup/memory/memory.limit_in_bytes"
	dockerCmd(c, "run", "--name", name, "-m", "300M", "busybox", "cat", file)
	dockerCmd(c, "update", "-m", "500M", name)

	assert.Equal(c, inspectField(c, name, "HostConfig.Memory"), "524288000")

	out, _ := dockerCmd(c, "start", "-a", name)
	assert.Equal(c, strings.TrimSpace(out), "524288000")
}

func (s *DockerSuite) TestUpdatePausedContainer(c *testing.T) {
	testRequires(c, DaemonIsLinux)
	testRequires(c, cpuShare)

	name := "test-update-container"
	dockerCmd(c, "run", "-d", "--name", name, "--cpu-shares", "1000", "busybox", "top")
	dockerCmd(c, "pause", name)
	dockerCmd(c, "update", "--cpu-shares", "500", name)

	assert.Equal(c, inspectField(c, name, "HostConfig.CPUShares"), "500")

	dockerCmd(c, "unpause", name)
	file := "/sys/fs/cgroup/cpu/cpu.shares"
	out, _ := dockerCmd(c, "exec", name, "cat", file)
	assert.Equal(c, strings.TrimSpace(out), "500")
}

func (s *DockerSuite) TestUpdateWithUntouchedFields(c *testing.T) {
	testRequires(c, DaemonIsLinux)
	testRequires(c, memoryLimitSupport)
	testRequires(c, cpuShare)

	name := "test-update-container"
	dockerCmd(c, "run", "-d", "--name", name, "-m", "300M", "--cpu-shares", "800", "busybox", "top")
	dockerCmd(c, "update", "-m", "500M", name)

	// Update memory and not touch cpus, `cpuset.cpus` should still have the old value
	out := inspectField(c, name, "HostConfig.CPUShares")
	assert.Equal(c, out, "800")

	file := "/sys/fs/cgroup/cpu/cpu.shares"
	out, _ = dockerCmd(c, "exec", name, "cat", file)
	assert.Equal(c, strings.TrimSpace(out), "800")
}

func (s *DockerSuite) TestUpdateContainerInvalidValue(c *testing.T) {
	testRequires(c, DaemonIsLinux)
	testRequires(c, memoryLimitSupport)

	name := "test-update-container"
	dockerCmd(c, "run", "-d", "--name", name, "-m", "300M", "busybox", "true")
	out, _, err := dockerCmdWithError("update", "-m", "2M", name)
	assert.ErrorContains(c, err, "")
	expected := "Minimum memory limit allowed is 6MB"
	assert.Assert(c, strings.Contains(out, expected))
}

func (s *DockerSuite) TestUpdateContainerWithoutFlags(c *testing.T) {
	testRequires(c, DaemonIsLinux)
	testRequires(c, memoryLimitSupport)

	name := "test-update-container"
	dockerCmd(c, "run", "-d", "--name", name, "-m", "300M", "busybox", "true")
	_, _, err := dockerCmdWithError("update", name)
	assert.ErrorContains(c, err, "")
}

func (s *DockerSuite) TestUpdateSwapMemoryOnly(c *testing.T) {
	testRequires(c, DaemonIsLinux)
	testRequires(c, memoryLimitSupport)
	testRequires(c, swapMemorySupport)

	name := "test-update-container"
	dockerCmd(c, "run", "-d", "--name", name, "--memory", "300M", "--memory-swap", "500M", "busybox", "top")
	dockerCmd(c, "update", "--memory-swap", "600M", name)

	assert.Equal(c, inspectField(c, name, "HostConfig.MemorySwap"), "629145600")

	file := "/sys/fs/cgroup/memory/memory.memsw.limit_in_bytes"
	out, _ := dockerCmd(c, "exec", name, "cat", file)
	assert.Equal(c, strings.TrimSpace(out), "629145600")
}

func (s *DockerSuite) TestUpdateInvalidSwapMemory(c *testing.T) {
	testRequires(c, DaemonIsLinux)
	testRequires(c, memoryLimitSupport)
	testRequires(c, swapMemorySupport)

	name := "test-update-container"
	dockerCmd(c, "run", "-d", "--name", name, "--memory", "300M", "--memory-swap", "500M", "busybox", "top")
	_, _, err := dockerCmdWithError("update", "--memory-swap", "200M", name)
	// Update invalid swap memory should fail.
	// This will pass docker config validation, but failed at kernel validation
	assert.ErrorContains(c, err, "")

	// Update invalid swap memory with failure should not change HostConfig
	assert.Equal(c, inspectField(c, name, "HostConfig.Memory"), "314572800")
	assert.Equal(c, inspectField(c, name, "HostConfig.MemorySwap"), "524288000")

	dockerCmd(c, "update", "--memory-swap", "600M", name)

	assert.Equal(c, inspectField(c, name, "HostConfig.MemorySwap"), "629145600")

	file := "/sys/fs/cgroup/memory/memory.memsw.limit_in_bytes"
	out, _ := dockerCmd(c, "exec", name, "cat", file)
	assert.Equal(c, strings.TrimSpace(out), "629145600")
}

func (s *DockerSuite) TestUpdateStats(c *testing.T) {
	testRequires(c, DaemonIsLinux)
	testRequires(c, memoryLimitSupport)
	testRequires(c, cpuCfsQuota)
	name := "foo"
	dockerCmd(c, "run", "-d", "-ti", "--name", name, "-m", "500m", "busybox")

	assert.NilError(c, waitRun(name))

	getMemLimit := func(id string) uint64 {
		resp, body, err := request.Get(fmt.Sprintf("/containers/%s/stats?stream=false", id))
		assert.NilError(c, err)
		assert.Equal(c, resp.Header.Get("Content-Type"), "application/json")

		var v *types.Stats
		err = json.NewDecoder(body).Decode(&v)
		assert.NilError(c, err)
		body.Close()

		return v.MemoryStats.Limit
	}
	preMemLimit := getMemLimit(name)

	dockerCmd(c, "update", "--cpu-quota", "2000", name)

	curMemLimit := getMemLimit(name)
	assert.Equal(c, preMemLimit, curMemLimit)
}

func (s *DockerSuite) TestUpdateMemoryWithSwapMemory(c *testing.T) {
	testRequires(c, DaemonIsLinux)
	testRequires(c, memoryLimitSupport)
	testRequires(c, swapMemorySupport)

	name := "test-update-container"
	dockerCmd(c, "run", "-d", "--name", name, "--memory", "300M", "busybox", "top")
	out, _, err := dockerCmdWithError("update", "--memory", "800M", name)
	assert.ErrorContains(c, err, "")
	assert.Assert(c, strings.Contains(out, "Memory limit should be smaller than already set memoryswap limit"))

	dockerCmd(c, "update", "--memory", "800M", "--memory-swap", "1000M", name)
}

func (s *DockerSuite) TestUpdateNotAffectMonitorRestartPolicy(c *testing.T) {
	testRequires(c, DaemonIsLinux, cpuShare)

	out, _ := dockerCmd(c, "run", "-tid", "--restart=always", "busybox", "sh")
	id := strings.TrimSpace(out)
	dockerCmd(c, "update", "--cpu-shares", "512", id)

	cpty, tty, err := pty.Open()
	assert.NilError(c, err)
	defer cpty.Close()

	cmd := exec.Command(dockerBinary, "attach", id)
	cmd.Stdin = tty

	assert.NilError(c, cmd.Start())
	defer cmd.Process.Kill()

	_, err = cpty.Write([]byte("exit\n"))
	assert.NilError(c, err)

	assert.NilError(c, cmd.Wait())

	// container should restart again and keep running
	err = waitInspect(id, "{{.RestartCount}}", "1", 30*time.Second)
	assert.NilError(c, err)
	assert.NilError(c, waitRun(id))
}

func (s *DockerSuite) TestUpdateWithNanoCPUs(c *testing.T) {
	testRequires(c, cpuCfsQuota, cpuCfsPeriod)

	file1 := "/sys/fs/cgroup/cpu/cpu.cfs_quota_us"
	file2 := "/sys/fs/cgroup/cpu/cpu.cfs_period_us"

	out, _ := dockerCmd(c, "run", "-d", "--cpus", "0.5", "--name", "top", "busybox", "top")
	assert.Assert(c, strings.TrimSpace(out) != "")

	out, _ = dockerCmd(c, "exec", "top", "sh", "-c", fmt.Sprintf("cat %s && cat %s", file1, file2))
	assert.Equal(c, strings.TrimSpace(out), "50000\n100000")

	clt, err := client.NewClientWithOpts(client.FromEnv)
	assert.NilError(c, err)
	inspect, err := clt.ContainerInspect(context.Background(), "top")
	assert.NilError(c, err)
	assert.Equal(c, inspect.HostConfig.NanoCPUs, int64(500000000))

	out = inspectField(c, "top", "HostConfig.CpuQuota")
	assert.Equal(c, out, "0", "CPU CFS quota should be 0")
	out = inspectField(c, "top", "HostConfig.CpuPeriod")
	assert.Equal(c, out, "0", "CPU CFS period should be 0")

	out, _, err = dockerCmdWithError("update", "--cpu-quota", "80000", "top")
	assert.ErrorContains(c, err, "")
	assert.Assert(c, strings.Contains(out, "Conflicting options: CPU Quota cannot be updated as NanoCPUs has already been set"))

	dockerCmd(c, "update", "--cpus", "0.8", "top")
	inspect, err = clt.ContainerInspect(context.Background(), "top")
	assert.NilError(c, err)
	assert.Equal(c, inspect.HostConfig.NanoCPUs, int64(800000000))

	out = inspectField(c, "top", "HostConfig.CpuQuota")
	assert.Equal(c, out, "0", "CPU CFS quota should be 0")
	out = inspectField(c, "top", "HostConfig.CpuPeriod")
	assert.Equal(c, out, "0", "CPU CFS period should be 0")

	out, _ = dockerCmd(c, "exec", "top", "sh", "-c", fmt.Sprintf("cat %s && cat %s", file1, file2))
	assert.Equal(c, strings.TrimSpace(out), "80000\n100000")
}
