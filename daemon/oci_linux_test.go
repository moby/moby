package daemon // import "github.com/docker/docker/daemon"

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/container"
	"github.com/docker/docker/daemon/config"
	"github.com/docker/docker/daemon/network"
	"github.com/docker/docker/oci"
	"github.com/docker/docker/pkg/containerfs"
	"github.com/docker/docker/pkg/idtools"
	"github.com/docker/libnetwork"
	"gotest.tools/assert"
	is "gotest.tools/assert/cmp"
)

func setupFakeDaemon(t *testing.T, c *container.Container) *Daemon {
	root, err := ioutil.TempDir("", "oci_linux_test-root")
	assert.NilError(t, err)

	rootfs := filepath.Join(root, "rootfs")
	err = os.MkdirAll(rootfs, 0755)
	assert.NilError(t, err)

	netController, err := libnetwork.New()
	assert.NilError(t, err)

	d := &Daemon{
		// some empty structs to avoid getting a panic
		// caused by a null pointer dereference
		idMapping:     &idtools.IdentityMapping{},
		configStore:   &config.Config{},
		linkIndex:     newLinkIndex(),
		netController: netController,
	}

	c.Root = root
	c.BaseFS = containerfs.NewLocalContainerFS(rootfs)

	if c.Config == nil {
		c.Config = new(containertypes.Config)
	}
	if c.HostConfig == nil {
		c.HostConfig = new(containertypes.HostConfig)
	}
	if c.NetworkSettings == nil {
		c.NetworkSettings = &network.Settings{Networks: make(map[string]*network.EndpointSettings)}
	}

	return d
}

func cleanupFakeContainer(c *container.Container) {
	os.RemoveAll(c.Root)
}

// TestTmpfsDevShmNoDupMount checks that a user-specified /dev/shm tmpfs
// mount (as in "docker run --tmpfs /dev/shm:rw,size=NNN") does not result
// in "Duplicate mount point" error from the engine.
// https://github.com/moby/moby/issues/35455
func TestTmpfsDevShmNoDupMount(t *testing.T) {
	c := &container.Container{
		ShmPath: "foobar", // non-empty, for c.IpcMounts() to work
		HostConfig: &containertypes.HostConfig{
			IpcMode: containertypes.IpcMode("shareable"), // default mode
			// --tmpfs /dev/shm:rw,exec,size=NNN
			Tmpfs: map[string]string{
				"/dev/shm": "rw,exec,size=1g",
			},
		},
	}
	d := setupFakeDaemon(t, c)
	defer cleanupFakeContainer(c)

	_, err := d.createSpec(c)
	assert.Check(t, err)
}

// TestIpcPrivateVsReadonly checks that in case of IpcMode: private
// and ReadonlyRootfs: true (as in "docker run --ipc private --read-only")
// the resulting /dev/shm mount is NOT made read-only.
// https://github.com/moby/moby/issues/36503
func TestIpcPrivateVsReadonly(t *testing.T) {
	c := &container.Container{
		HostConfig: &containertypes.HostConfig{
			IpcMode:        containertypes.IpcMode("private"),
			ReadonlyRootfs: true,
		},
	}
	d := setupFakeDaemon(t, c)
	defer cleanupFakeContainer(c)

	s, err := d.createSpec(c)
	assert.Check(t, err)

	// Find the /dev/shm mount in ms, check it does not have ro
	for _, m := range s.Mounts {
		if m.Destination != "/dev/shm" {
			continue
		}
		assert.Check(t, is.Equal(false, inSlice(m.Options, "ro")))
	}
}

// TestSysctlOverride ensures that any implicit sysctls (such as
// Config.Domainname) are overridden by an explicit sysctl in the HostConfig.
func TestSysctlOverride(t *testing.T) {
	c := &container.Container{
		Config: &containertypes.Config{
			Hostname:   "foobar",
			Domainname: "baz.cyphar.com",
		},
		HostConfig: &containertypes.HostConfig{
			Sysctls: map[string]string{},
		},
	}
	d := setupFakeDaemon(t, c)
	defer cleanupFakeContainer(c)

	// Ensure that the implicit sysctl is set correctly.
	s, err := d.createSpec(c)
	assert.NilError(t, err)
	assert.Equal(t, s.Hostname, "foobar")
	assert.Equal(t, s.Linux.Sysctl["kernel.domainname"], c.Config.Domainname)

	// Set an explicit sysctl.
	c.HostConfig.Sysctls["kernel.domainname"] = "foobar.net"
	assert.Assert(t, c.HostConfig.Sysctls["kernel.domainname"] != c.Config.Domainname)

	s, err = d.createSpec(c)
	assert.NilError(t, err)
	assert.Equal(t, s.Hostname, "foobar")
	assert.Equal(t, s.Linux.Sysctl["kernel.domainname"], c.HostConfig.Sysctls["kernel.domainname"])
}

func TestGetSourceMount(t *testing.T) {
	// must be able to find source mount for /
	mnt, _, err := getSourceMount("/")
	assert.NilError(t, err)
	assert.Equal(t, mnt, "/")

	// must be able to find source mount for current directory
	cwd, err := os.Getwd()
	assert.NilError(t, err)
	_, _, err = getSourceMount(cwd)
	assert.NilError(t, err)
}

// TestMountsHaveWriteMode checks that every mount has a write-mode set (either 'ro' or 'rw')
func TestMountsHaveWriteMode(t *testing.T) {
	d := Daemon{idMapping: &idtools.IdentityMapping{}}
	c := &container.Container{HostConfig: &containertypes.HostConfig{}}

	spec := oci.DefaultSpec()
	err := setMounts(&d, &spec, c, []container.Mount{})
	assert.Check(t, err)

	for _, m := range spec.Mounts {
		hasWriteMode := inSlice(m.Options, "rw") || inSlice(m.Options, "ro")
		assert.Check(t, hasWriteMode, "expected mount to have 'ro' or 'rw': %s", m.Options)
	}
}

func TestSetMount(t *testing.T) {
	d := Daemon{idMapping: &idtools.IdentityMapping{}}
	c := &container.Container{HostConfig: &containertypes.HostConfig{}}
	spec := oci.DefaultSpec()

	tests := []struct {
		name       string
		hc         containertypes.HostConfig
		userMounts []container.Mount
		expected   map[string][]string
	}{
		{
			name: "default",
			userMounts: []container.Mount{
				{
					Source:      "/readonly",
					Destination: "/readonly",
					Writable:    false,
				},
				{
					Source:      "/writable",
					Destination: "/writable",
					Writable:    true,
				},
			},
			expected: map[string][]string{
				"/readonly":      {"ro"},
				"/writable":      {"rw"},
				"/proc":          {"rw"},
				"/dev":           {"rw"},
				"/dev/pts":       {"rw"},
				"/sys":           {"ro"},
				"/sys/fs/cgroup": {"ro"},
				"/dev/mqueue":    {"rw"},
				"/dev/shm":       {"rw"},
			},
		},
		{
			name: "privileged",
			hc:   containertypes.HostConfig{Privileged: true},
			expected: map[string][]string{
				"/proc":          {"rw"},
				"/dev":           {"rw"},
				"/dev/pts":       {"rw"},
				"/sys":           {"rw"},
				"/sys/fs/cgroup": {"rw"},
				"/dev/mqueue":    {"rw"},
				"/dev/shm":       {"rw"},
			},
		},
		{
			name: "readonly",
			hc:   containertypes.HostConfig{ReadonlyRootfs: true},
			userMounts: []container.Mount{
				{
					Source:      "/readonly",
					Destination: "/readonly",
					Writable:    false,
				},
				{
					Source:      "/writable",
					Destination: "/writable",
					Writable:    true,
				},
			},
			expected: map[string][]string{
				"/readonly":      {"ro"},
				"/writable":      {"rw"},
				"/proc":          {"rw"},
				"/dev":           {"rw"},
				"/dev/pts":       {"rw"},
				"/sys":           {"ro"},
				"/sys/fs/cgroup": {"ro"},
				"/dev/mqueue":    {"rw"},
				"/dev/shm":       {"rw"},
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c.HostConfig = &tc.hc
			s := spec
			if c.HostConfig.ReadonlyRootfs {
				// TODO why does setMounts look at spec.Root.Readonly, and not at HostConfig?
				s.Root.Readonly = true
			}
			err := setMounts(&d, &s, c, []container.Mount{})
			assert.Check(t, err)

			for _, m := range spec.Mounts {
				if expectedOpts, ok := tc.expected[m.Destination]; ok {
					for _, opt := range expectedOpts {
						assert.Check(t, inSlice(m.Options, opt), "option %q not found for mount %q", opt, m.Destination)
					}
				}
			}

		})
	}
}

// inSlice tests whether a string is contained in a slice of strings or not.
// Comparison is case sensitive
func inSlice(slice []string, s string) bool {
	for _, ss := range slice {
		if s == ss {
			return true
		}
	}
	return false
}
