//go:build !windows

package daemon // import "github.com/docker/docker/daemon"

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/docker/docker/api/types/blkiodev"
	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/container"
	"github.com/docker/docker/daemon/config"
	"github.com/docker/docker/pkg/sysinfo"
	"github.com/opencontainers/selinux/go-selinux"
	"golang.org/x/sys/unix"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

type fakeContainerGetter struct {
	containers map[string]*container.Container
}

func (f *fakeContainerGetter) GetContainer(cid string) (*container.Container, error) {
	ctr, ok := f.containers[cid]
	if !ok {
		return nil, errors.New("container not found")
	}
	return ctr, nil
}

// Unix test as uses settings which are not available on Windows
func TestAdjustSharedNamespaceContainerName(t *testing.T) {
	fakeID := "abcdef1234567890"
	hostConfig := &containertypes.HostConfig{
		IpcMode:     containertypes.IpcMode("container:base"),
		PidMode:     containertypes.PidMode("container:base"),
		NetworkMode: containertypes.NetworkMode("container:base"),
	}
	containerStore := &fakeContainerGetter{}
	containerStore.containers = make(map[string]*container.Container)
	containerStore.containers["base"] = &container.Container{
		ID: fakeID,
	}

	adaptSharedNamespaceContainer(containerStore, hostConfig)
	if hostConfig.IpcMode != containertypes.IpcMode("container:"+fakeID) {
		t.Errorf("Expected IpcMode to be container:%s", fakeID)
	}
	if hostConfig.PidMode != containertypes.PidMode("container:"+fakeID) {
		t.Errorf("Expected PidMode to be container:%s", fakeID)
	}
	if hostConfig.NetworkMode != containertypes.NetworkMode("container:"+fakeID) {
		t.Errorf("Expected NetworkMode to be container:%s", fakeID)
	}
}

// Unix test as uses settings which are not available on Windows
func TestAdjustCPUShares(t *testing.T) {
	tmp, err := os.MkdirTemp("", "docker-daemon-unix-test-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmp)
	daemon := &Daemon{
		repository: tmp,
		root:       tmp,
	}
	cfg := &config.Config{}
	muteLogs()

	hostConfig := &containertypes.HostConfig{
		Resources: containertypes.Resources{CPUShares: linuxMinCPUShares - 1},
	}
	daemon.adaptContainerSettings(cfg, hostConfig, true)
	if hostConfig.CPUShares != linuxMinCPUShares {
		t.Errorf("Expected CPUShares to be %d", linuxMinCPUShares)
	}

	hostConfig.CPUShares = linuxMaxCPUShares + 1
	daemon.adaptContainerSettings(cfg, hostConfig, true)
	if hostConfig.CPUShares != linuxMaxCPUShares {
		t.Errorf("Expected CPUShares to be %d", linuxMaxCPUShares)
	}

	hostConfig.CPUShares = 0
	daemon.adaptContainerSettings(cfg, hostConfig, true)
	if hostConfig.CPUShares != 0 {
		t.Error("Expected CPUShares to be unchanged")
	}

	hostConfig.CPUShares = 1024
	daemon.adaptContainerSettings(cfg, hostConfig, true)
	if hostConfig.CPUShares != 1024 {
		t.Error("Expected CPUShares to be unchanged")
	}
}

// Unix test as uses settings which are not available on Windows
func TestAdjustCPUSharesNoAdjustment(t *testing.T) {
	tmp, err := os.MkdirTemp("", "docker-daemon-unix-test-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmp)
	daemon := &Daemon{
		repository: tmp,
		root:       tmp,
	}
	cfg := &config.Config{}

	hostConfig := &containertypes.HostConfig{
		Resources: containertypes.Resources{CPUShares: linuxMinCPUShares - 1},
	}
	daemon.adaptContainerSettings(cfg, hostConfig, false)
	if hostConfig.CPUShares != linuxMinCPUShares-1 {
		t.Errorf("Expected CPUShares to be %d", linuxMinCPUShares-1)
	}

	hostConfig.CPUShares = linuxMaxCPUShares + 1
	daemon.adaptContainerSettings(cfg, hostConfig, false)
	if hostConfig.CPUShares != linuxMaxCPUShares+1 {
		t.Errorf("Expected CPUShares to be %d", linuxMaxCPUShares+1)
	}

	hostConfig.CPUShares = 0
	daemon.adaptContainerSettings(cfg, hostConfig, false)
	if hostConfig.CPUShares != 0 {
		t.Error("Expected CPUShares to be unchanged")
	}

	hostConfig.CPUShares = 1024
	daemon.adaptContainerSettings(cfg, hostConfig, false)
	if hostConfig.CPUShares != 1024 {
		t.Error("Expected CPUShares to be unchanged")
	}
}

// Unix test as uses settings which are not available on Windows
func TestParseSecurityOptWithDeprecatedColon(t *testing.T) {
	opts := &container.SecurityOptions{}
	cfg := &containertypes.HostConfig{}

	// test apparmor
	cfg.SecurityOpt = []string{"apparmor=test_profile"}
	if err := parseSecurityOpt(opts, cfg); err != nil {
		t.Fatalf("Unexpected parseSecurityOpt error: %v", err)
	}
	if opts.AppArmorProfile != "test_profile" {
		t.Fatalf("Unexpected AppArmorProfile, expected: \"test_profile\", got %q", opts.AppArmorProfile)
	}

	// test seccomp
	sp := "/path/to/seccomp_test.json"
	cfg.SecurityOpt = []string{"seccomp=" + sp}
	if err := parseSecurityOpt(opts, cfg); err != nil {
		t.Fatalf("Unexpected parseSecurityOpt error: %v", err)
	}
	if opts.SeccompProfile != sp {
		t.Fatalf("Unexpected AppArmorProfile, expected: %q, got %q", sp, opts.SeccompProfile)
	}

	// test valid label
	cfg.SecurityOpt = []string{"label=user:USER"}
	if err := parseSecurityOpt(opts, cfg); err != nil {
		t.Fatalf("Unexpected parseSecurityOpt error: %v", err)
	}

	// test invalid label
	cfg.SecurityOpt = []string{"label"}
	if err := parseSecurityOpt(opts, cfg); err == nil {
		t.Fatal("Expected parseSecurityOpt error, got nil")
	}

	// test invalid opt
	cfg.SecurityOpt = []string{"test"}
	if err := parseSecurityOpt(opts, cfg); err == nil {
		t.Fatal("Expected parseSecurityOpt error, got nil")
	}
}

func TestParseSecurityOpt(t *testing.T) {
	t.Run("apparmor", func(t *testing.T) {
		secOpts := &container.SecurityOptions{}
		err := parseSecurityOpt(secOpts, &containertypes.HostConfig{
			SecurityOpt: []string{"apparmor=test_profile"},
		})
		assert.Check(t, err)
		assert.Equal(t, secOpts.AppArmorProfile, "test_profile")
	})
	t.Run("apparmor using legacy separator", func(t *testing.T) {
		secOpts := &container.SecurityOptions{}
		err := parseSecurityOpt(secOpts, &containertypes.HostConfig{
			SecurityOpt: []string{"apparmor:test_profile"},
		})
		assert.Check(t, err)
		assert.Equal(t, secOpts.AppArmorProfile, "test_profile")
	})
	t.Run("seccomp", func(t *testing.T) {
		secOpts := &container.SecurityOptions{}
		err := parseSecurityOpt(secOpts, &containertypes.HostConfig{
			SecurityOpt: []string{"seccomp=/path/to/seccomp_test.json"},
		})
		assert.Check(t, err)
		assert.Equal(t, secOpts.SeccompProfile, "/path/to/seccomp_test.json")
	})
	t.Run("valid label", func(t *testing.T) {
		secOpts := &container.SecurityOptions{}
		err := parseSecurityOpt(secOpts, &containertypes.HostConfig{
			SecurityOpt: []string{"label=user:USER"},
		})
		assert.Check(t, err)
		if selinux.GetEnabled() {
			// TODO(thaJeztah): set expected labels here (or "partial" if depends on host)
			// assert.Check(t, is.Equal(secOpts.MountLabel, ""))
			// assert.Check(t, is.Equal(secOpts.ProcessLabel, ""))
		} else {
			assert.Check(t, is.Equal(secOpts.MountLabel, ""))
			assert.Check(t, is.Equal(secOpts.ProcessLabel, ""))
		}
	})
	t.Run("invalid label", func(t *testing.T) {
		secOpts := &container.SecurityOptions{}
		err := parseSecurityOpt(secOpts, &containertypes.HostConfig{
			SecurityOpt: []string{"label"},
		})
		assert.Error(t, err, `invalid --security-opt 1: "label"`)
	})
	t.Run("invalid option (no value)", func(t *testing.T) {
		secOpts := &container.SecurityOptions{}
		err := parseSecurityOpt(secOpts, &containertypes.HostConfig{
			SecurityOpt: []string{"unknown"},
		})
		assert.Error(t, err, `invalid --security-opt 1: "unknown"`)
	})
	t.Run("unknown option", func(t *testing.T) {
		secOpts := &container.SecurityOptions{}
		err := parseSecurityOpt(secOpts, &containertypes.HostConfig{
			SecurityOpt: []string{"unknown=something"},
		})
		assert.Error(t, err, `invalid --security-opt 2: "unknown=something"`)
	})
}

func TestParseNNPSecurityOptions(t *testing.T) {
	daemonCfg := &config.Config{NoNewPrivileges: true}
	daemon := &Daemon{}
	daemon.configStore.Store(daemonCfg)
	opts := &container.SecurityOptions{}
	cfg := &containertypes.HostConfig{}

	// test NNP when "daemon:true" and "no-new-privileges=false""
	cfg.SecurityOpt = []string{"no-new-privileges=false"}

	if err := daemon.parseSecurityOpt(daemonCfg, opts, cfg); err != nil {
		t.Fatalf("Unexpected daemon.parseSecurityOpt error: %v", err)
	}
	if opts.NoNewPrivileges {
		t.Fatalf("container.NoNewPrivileges should be FALSE: %v", opts.NoNewPrivileges)
	}

	// test NNP when "daemon:false" and "no-new-privileges=true""
	daemonCfg.NoNewPrivileges = false
	cfg.SecurityOpt = []string{"no-new-privileges=true"}

	if err := daemon.parseSecurityOpt(daemonCfg, opts, cfg); err != nil {
		t.Fatalf("Unexpected daemon.parseSecurityOpt error: %v", err)
	}
	if !opts.NoNewPrivileges {
		t.Fatalf("container.NoNewPrivileges should be TRUE: %v", opts.NoNewPrivileges)
	}
}

func TestVerifyPlatformContainerResources(t *testing.T) {
	t.Parallel()
	var (
		no  = false
		yes = true
	)

	withMemoryLimit := func(si *sysinfo.SysInfo) {
		si.MemoryLimit = true
	}
	withSwapLimit := func(si *sysinfo.SysInfo) {
		si.SwapLimit = true
	}
	withOomKillDisable := func(si *sysinfo.SysInfo) {
		si.OomKillDisable = true
	}

	tests := []struct {
		name             string
		resources        containertypes.Resources
		sysInfo          sysinfo.SysInfo
		update           bool
		expectedWarnings []string
	}{
		{
			name:             "no-oom-kill-disable",
			resources:        containertypes.Resources{},
			sysInfo:          sysInfo(t, withMemoryLimit),
			expectedWarnings: []string{},
		},
		{
			name: "oom-kill-disable-disabled",
			resources: containertypes.Resources{
				OomKillDisable: &no,
			},
			sysInfo:          sysInfo(t, withMemoryLimit),
			expectedWarnings: []string{},
		},
		{
			name: "oom-kill-disable-not-supported",
			resources: containertypes.Resources{
				OomKillDisable: &yes,
			},
			sysInfo: sysInfo(t, withMemoryLimit),
			expectedWarnings: []string{
				"Your kernel does not support OomKillDisable. OomKillDisable discarded.",
			},
		},
		{
			name: "oom-kill-disable-without-memory-constraints",
			resources: containertypes.Resources{
				OomKillDisable: &yes,
				Memory:         0,
			},
			sysInfo: sysInfo(t, withMemoryLimit, withOomKillDisable, withSwapLimit),
			expectedWarnings: []string{
				"OOM killer is disabled for the container, but no memory limit is set, this can result in the system running out of resources.",
			},
		},
		{
			name: "oom-kill-disable-with-memory-constraints-but-no-memory-limit-support",
			resources: containertypes.Resources{
				OomKillDisable: &yes,
				Memory:         linuxMinMemory,
			},
			sysInfo: sysInfo(t, withOomKillDisable),
			expectedWarnings: []string{
				"Your kernel does not support memory limit capabilities or the cgroup is not mounted. Limitation discarded.",
				"OOM killer is disabled for the container, but no memory limit is set, this can result in the system running out of resources.",
			},
		},
		{
			name: "oom-kill-disable-with-memory-constraints",
			resources: containertypes.Resources{
				OomKillDisable: &yes,
				Memory:         linuxMinMemory,
			},
			sysInfo:          sysInfo(t, withMemoryLimit, withOomKillDisable, withSwapLimit),
			expectedWarnings: []string{},
		},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			warnings, err := verifyPlatformContainerResources(&tc.resources, &tc.sysInfo, tc.update)
			assert.NilError(t, err)
			for _, w := range tc.expectedWarnings {
				assert.Assert(t, is.Contains(warnings, w))
			}
		})
	}
}

func sysInfo(t *testing.T, opts ...func(*sysinfo.SysInfo)) sysinfo.SysInfo {
	t.Helper()
	si := sysinfo.SysInfo{}

	for _, opt := range opts {
		opt(&si)
	}

	if si.OomKillDisable {
		t.Log(t.Name(), "OOM disable supported")
	}
	return si
}

const (
	// prepare major 0x1FD(509 in decimal) and minor 0x130(304)
	DEVNO  = 0x11FD30
	MAJOR  = 509
	MINOR  = 304
	WEIGHT = 1024
)

func deviceTypeMock(t *testing.T, testAndCheck func(string)) {
	if os.Getuid() != 0 {
		t.Skip("root required") // for mknod
	}

	t.Parallel()

	tempDir, err := os.MkdirTemp("", "tempDevDir"+t.Name())
	assert.NilError(t, err, "create temp file")
	tempFile := filepath.Join(tempDir, "dev")

	defer os.RemoveAll(tempDir)

	if err = unix.Mknod(tempFile, unix.S_IFCHR, DEVNO); err != nil {
		t.Fatalf("mknod error %s(%x): %v", tempFile, DEVNO, err)
	}

	testAndCheck(tempFile)
}

func TestGetBlkioWeightDevices(t *testing.T) {
	deviceTypeMock(t, func(tempFile string) {
		mockResource := containertypes.Resources{
			BlkioWeightDevice: []*blkiodev.WeightDevice{{Path: tempFile, Weight: WEIGHT}},
		}

		weightDevs, err := getBlkioWeightDevices(mockResource)

		assert.NilError(t, err, "getBlkioWeightDevices")
		assert.Check(t, is.Len(weightDevs, 1), "getBlkioWeightDevices")
		assert.Check(t, weightDevs[0].Major == MAJOR, "get major device type")
		assert.Check(t, weightDevs[0].Minor == MINOR, "get minor device type")
		assert.Check(t, *weightDevs[0].Weight == WEIGHT, "get device weight")
	})
}

func TestGetBlkioThrottleDevices(t *testing.T) {
	deviceTypeMock(t, func(tempFile string) {
		mockDevs := []*blkiodev.ThrottleDevice{{Path: tempFile, Rate: WEIGHT}}

		retDevs, err := getBlkioThrottleDevices(mockDevs)

		assert.NilError(t, err, "getBlkioThrottleDevices")
		assert.Check(t, is.Len(retDevs, 1), "getBlkioThrottleDevices")
		assert.Check(t, retDevs[0].Major == MAJOR, "get major device type")
		assert.Check(t, retDevs[0].Minor == MINOR, "get minor device type")
		assert.Check(t, retDevs[0].Rate == WEIGHT, "get device rate")
	})
}
