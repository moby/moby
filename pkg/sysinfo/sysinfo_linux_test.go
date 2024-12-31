package sysinfo // import "github.com/docker/docker/pkg/sysinfo"

import (
	"os"
	"path"
	"path/filepath"
	"reflect"
	"testing"

	"golang.org/x/sys/unix"
	"gotest.tools/v3/assert"
)

func TestReadProcBool(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test-sysinfo-proc")
	assert.NilError(t, err)
	defer os.RemoveAll(tmpDir)

	procFile := filepath.Join(tmpDir, "read-proc-bool")
	err = os.WriteFile(procFile, []byte("1"), 0o644)
	assert.NilError(t, err)

	if !readProcBool(procFile) {
		t.Fatal("expected proc bool to be true, got false")
	}

	if err := os.WriteFile(procFile, []byte("0"), 0o644); err != nil {
		t.Fatal(err)
	}
	if readProcBool(procFile) {
		t.Fatal("expected proc bool to be false, got true")
	}

	if readProcBool(path.Join(tmpDir, "no-exist")) {
		t.Fatal("should be false for non-existent entry")
	}
}

func TestCgroupEnabled(t *testing.T) {
	cgroupDir, err := os.MkdirTemp("", "cgroup-test")
	assert.NilError(t, err)
	defer os.RemoveAll(cgroupDir)

	if cgroupEnabled(cgroupDir, "test") {
		t.Fatal("cgroupEnabled should be false")
	}

	err = os.WriteFile(path.Join(cgroupDir, "test"), []byte{}, 0o644)
	assert.NilError(t, err)

	if !cgroupEnabled(cgroupDir, "test") {
		t.Fatal("cgroupEnabled should be true")
	}
}

func TestNew(t *testing.T) {
	sysInfo := New()
	assert.Assert(t, sysInfo != nil)
	checkSysInfo(t, sysInfo)
}

func checkSysInfo(t *testing.T, sysInfo *SysInfo) {
	// Check if Seccomp is supported, via CONFIG_SECCOMP.then sysInfo.Seccomp must be TRUE , else FALSE
	if err := unix.Prctl(unix.PR_GET_SECCOMP, 0, 0, 0, 0); err != unix.EINVAL {
		// Make sure the kernel has CONFIG_SECCOMP_FILTER.
		if err := unix.Prctl(unix.PR_SET_SECCOMP, unix.SECCOMP_MODE_FILTER, 0, 0, 0); err != unix.EINVAL {
			assert.Assert(t, sysInfo.Seccomp)
		}
	} else {
		assert.Assert(t, !sysInfo.Seccomp)
	}
}

func TestNewAppArmorEnabled(t *testing.T) {
	// Check if AppArmor is supported. then it must be TRUE , else FALSE
	if _, err := os.Stat("/sys/kernel/security/apparmor"); err != nil {
		t.Skip("AppArmor Must be Enabled")
	}

	sysInfo := New()
	assert.Assert(t, sysInfo.AppArmor)
}

func TestNewAppArmorDisabled(t *testing.T) {
	// Check if AppArmor is supported. then it must be TRUE , else FALSE
	if _, err := os.Stat("/sys/kernel/security/apparmor"); !os.IsNotExist(err) {
		t.Skip("AppArmor Must be Disabled")
	}

	sysInfo := New()
	assert.Assert(t, !sysInfo.AppArmor)
}

func TestNewCgroupNamespacesEnabled(t *testing.T) {
	// If cgroup namespaces are supported in the kernel, then sysInfo.CgroupNamespaces should be TRUE
	if _, err := os.Stat("/proc/self/ns/cgroup"); err != nil {
		t.Skip("cgroup namespaces must be enabled")
	}

	sysInfo := New()
	assert.Assert(t, sysInfo.CgroupNamespaces)
}

func TestNewCgroupNamespacesDisabled(t *testing.T) {
	// If cgroup namespaces are *not* supported in the kernel, then sysInfo.CgroupNamespaces should be FALSE
	if _, err := os.Stat("/proc/self/ns/cgroup"); !os.IsNotExist(err) {
		t.Skip("cgroup namespaces must be disabled")
	}

	sysInfo := New()
	assert.Assert(t, !sysInfo.CgroupNamespaces)
}

func TestNumCPU(t *testing.T) {
	cpuNumbers := NumCPU()
	if cpuNumbers <= 0 {
		t.Fatal("CPU returned must be greater than zero")
	}
}

func TestIsCpusetListAvailable(t *testing.T) {
	cases := []struct {
		provided  string
		available string
		res       bool
		err       bool
	}{
		{"1", "0-4", true, false},
		{"01,3", "0-4", true, false},
		{"", "0-7", true, false},
		{"1--42", "0-7", false, true},
		{"1-42", "00-1,8,,9", false, true},
		{"1,41-42", "43,45", false, false},
		{"0-3", "", false, false},
	}
	for _, c := range cases {
		r, err := isCpusetListAvailable(c.provided, c.available)
		if (c.err && err == nil) && r != c.res {
			t.Fatalf("Expected pair: %v, %v for %s, %s. Got %v, %v instead", c.res, c.err, c.provided, c.available, (c.err && err == nil), r)
		}
	}
}

func TestParseUintList(t *testing.T) {
	valids := map[string]map[int]bool{
		"":             {},
		"7":            {7: true},
		"1-6":          {1: true, 2: true, 3: true, 4: true, 5: true, 6: true},
		"0-7":          {0: true, 1: true, 2: true, 3: true, 4: true, 5: true, 6: true, 7: true},
		"0,3-4,7,8-10": {0: true, 3: true, 4: true, 7: true, 8: true, 9: true, 10: true},
		"0-0,0,1-4":    {0: true, 1: true, 2: true, 3: true, 4: true},
		"03,1-3":       {1: true, 2: true, 3: true},
		"3,2,1":        {1: true, 2: true, 3: true},
		"0-2,3,1":      {0: true, 1: true, 2: true, 3: true},
	}
	for k, v := range valids {
		out, err := parseUintList(k, 0)
		if err != nil {
			t.Fatalf("Expected not to fail, got %v", err)
		}
		if !reflect.DeepEqual(out, v) {
			t.Fatalf("Expected %v, got %v", v, out)
		}
	}

	invalids := []string{
		"this",
		"1--",
		"1-10,,10",
		"10-1",
		"-1",
		"-1,0",
	}
	for _, v := range invalids {
		if out, err := parseUintList(v, 0); err == nil {
			t.Fatalf("Expected failure with %s but got %v", v, out)
		}
	}
}

func TestParseUintListMaximumLimits(t *testing.T) {
	v := "10,1000"
	if _, err := parseUintList(v, 0); err != nil {
		t.Fatalf("Expected not to fail, got %v", err)
	}
	if _, err := parseUintList(v, 1000); err != nil {
		t.Fatalf("Expected not to fail, got %v", err)
	}
	if out, err := parseUintList(v, 100); err == nil {
		t.Fatalf("Expected failure with %s but got %v", v, out)
	}
}
