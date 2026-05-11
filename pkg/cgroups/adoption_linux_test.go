package cgroups

import (
	"os"
	"path/filepath"
	"testing"

	"gotest.tools/v3/assert"
)

func TestDeriveParentFromPid_Cgroupv2(t *testing.T) {
	// Test cgroup v2 format: "0::/user.slice/user-1000.slice/session-1.scope"
	tmpdir := t.TempDir()
	cgroupFile := filepath.Join(tmpdir, "cgroup")

	content := `0::/user.slice/user-1000.slice/session-1.scope
`
	err := os.WriteFile(cgroupFile, []byte(content), 0644)
	assert.NilError(t, err)

	parent, err := deriveParentFromCgroupFile(cgroupFile)
	assert.NilError(t, err)
	assert.Equal(t, parent, "user.slice/user-1000.slice/session-1.scope")
}

func TestDeriveParentFromPid_Cgroupv1_Systemd(t *testing.T) {
	// Test cgroup v1 format with systemd controller
	tmpdir := t.TempDir()
	cgroupFile := filepath.Join(tmpdir, "cgroup")

	content := `12:blkio:/user.slice/user-1000.slice/session-1.scope
11:pids:/user.slice/user-1000.slice/session-1.scope
10:memory:/user.slice/user-1000.slice/session-1.scope
9:perf_event:/
8:devices:/user.slice/user-1000.slice/session-1.scope
7:cpuset:/
6:freezer:/
5:net_cls,net_prio:/
4:cpu,cpuacct:/user.slice/user-1000.slice/session-1.scope
3:hugetlb:/
2:rdma:/
1:name=systemd:/user.slice/user-1000.slice/session-1.scope
`
	err := os.WriteFile(cgroupFile, []byte(content), 0644)
	assert.NilError(t, err)

	parent, err := deriveParentFromCgroupFile(cgroupFile)
	assert.NilError(t, err)
	assert.Equal(t, parent, "user.slice/user-1000.slice/session-1.scope")
}

func TestDeriveParentFromPid_MultipleSlices(t *testing.T) {
	// Test that we return the full cgroup path
	tmpdir := t.TempDir()
	cgroupFile := filepath.Join(tmpdir, "cgroup")

	content := `0::/system.slice/docker.service/user.slice/user-1000.slice/app.scope
`
	err := os.WriteFile(cgroupFile, []byte(content), 0644)
	assert.NilError(t, err)

	parent, err := deriveParentFromCgroupFile(cgroupFile)
	assert.NilError(t, err)
	// Should return the full path
	assert.Equal(t, parent, "system.slice/docker.service/user.slice/user-1000.slice/app.scope")
}

func TestDeriveParentFromPid_NoSlice(t *testing.T) {
	// Test that we return full path even without .slice components
	tmpdir := t.TempDir()
	cgroupFile := filepath.Join(tmpdir, "cgroup")

	content := `0::/docker/container-id
`
	err := os.WriteFile(cgroupFile, []byte(content), 0644)
	assert.NilError(t, err)

	parent, err := deriveParentFromCgroupFile(cgroupFile)
	assert.NilError(t, err)
	assert.Equal(t, parent, "docker/container-id")
}

func TestDeriveParentFromPid_RootCgroup(t *testing.T) {
	// Test root cgroup "/" - should return error
	tmpdir := t.TempDir()
	cgroupFile := filepath.Join(tmpdir, "cgroup")

	content := `0::/
`
	err := os.WriteFile(cgroupFile, []byte(content), 0644)
	assert.NilError(t, err)

	parent, err := deriveParentFromCgroupFile(cgroupFile)
	assert.Assert(t, err != nil, "should return error for root cgroup")
	assert.Equal(t, parent, "")
}

func TestDeriveParentFromPid_InvalidFile(t *testing.T) {
	parent, err := deriveParentFromCgroupFile("/nonexistent/file")
	assert.Assert(t, err != nil, "should return error for nonexistent file")
	assert.Equal(t, parent, "")
}

func TestDeriveParentFromPid_EmptyFile(t *testing.T) {
	tmpdir := t.TempDir()
	cgroupFile := filepath.Join(tmpdir, "cgroup")

	err := os.WriteFile(cgroupFile, []byte(""), 0644)
	assert.NilError(t, err)

	parent, err := deriveParentFromCgroupFile(cgroupFile)
	assert.Assert(t, err != nil, "should return error for empty file")
	assert.Equal(t, parent, "")
}

func TestDeriveParentFromPid_SlurmCgroup(t *testing.T) {
	// Test SLURM job cgroup hierarchy - must preserve full path
	tmpdir := t.TempDir()
	cgroupFile := filepath.Join(tmpdir, "cgroup")

	content := `0::/system.slice/slurmstepd.scope/job_298726/step_0/user/task_0
`
	err := os.WriteFile(cgroupFile, []byte(content), 0644)
	assert.NilError(t, err)

	parent, err := deriveParentFromCgroupFile(cgroupFile)
	assert.NilError(t, err)
	// Must return the full SLURM hierarchy, not just system.slice
	assert.Equal(t, parent, "system.slice/slurmstepd.scope/job_298726/step_0/user/task_0")
}
