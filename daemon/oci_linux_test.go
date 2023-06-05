package daemon // import "github.com/docker/docker/daemon"

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/container"
	"github.com/docker/docker/daemon/config"
	"github.com/docker/docker/daemon/network"
	"github.com/docker/docker/libnetwork"
	"golang.org/x/sys/unix"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/skip"
)

func setupFakeDaemon(t *testing.T, c *container.Container) *Daemon {
	t.Helper()
	root := t.TempDir()

	rootfs := filepath.Join(root, "rootfs")
	err := os.MkdirAll(rootfs, 0755)
	assert.NilError(t, err)

	netController, err := libnetwork.New()
	assert.NilError(t, err)

	d := &Daemon{
		// some empty structs to avoid getting a panic
		// caused by a null pointer dereference
		linkIndex:     newLinkIndex(),
		netController: netController,
		imageService:  &fakeImageService{},
	}

	c.Root = root
	c.BaseFS = rootfs

	if c.Config == nil {
		c.Config = new(containertypes.Config)
	}
	if c.HostConfig == nil {
		c.HostConfig = new(containertypes.HostConfig)
	}
	if c.NetworkSettings == nil {
		c.NetworkSettings = &network.Settings{Networks: make(map[string]*network.EndpointSettings)}
	}

	// HORRIBLE HACK: clean up shm mounts leaked by some tests. Otherwise the
	// offending tests would fail due to the mounts blocking the temporary
	// directory from being cleaned up.
	t.Cleanup(func() {
		if c.ShmPath != "" {
			var err error
			for err == nil { // Some tests over-mount over the same path multiple times.
				err = unix.Unmount(c.ShmPath, unix.MNT_DETACH)
			}
		}
	})

	return d
}

type fakeImageService struct {
	ImageService
}

func (i *fakeImageService) StorageDriver() string {
	return "overlay"
}

// TestTmpfsDevShmNoDupMount checks that a user-specified /dev/shm tmpfs
// mount (as in "docker run --tmpfs /dev/shm:rw,size=NNN") does not result
// in "Duplicate mount point" error from the engine.
// https://github.com/moby/moby/issues/35455
func TestTmpfsDevShmNoDupMount(t *testing.T) {
	skip.If(t, os.Getuid() != 0, "skipping test that requires root")
	c := &container.Container{
		ShmPath: "foobar", // non-empty, for c.IpcMounts() to work
		HostConfig: &containertypes.HostConfig{
			IpcMode: containertypes.IPCModeShareable, // default mode
			// --tmpfs /dev/shm:rw,exec,size=NNN
			Tmpfs: map[string]string{
				"/dev/shm": "rw,exec,size=1g",
			},
		},
	}
	d := setupFakeDaemon(t, c)

	_, err := d.createSpec(context.TODO(), &configStore{}, c)
	assert.Check(t, err)
}

// TestIpcPrivateVsReadonly checks that in case of IpcMode: private
// and ReadonlyRootfs: true (as in "docker run --ipc private --read-only")
// the resulting /dev/shm mount is NOT made read-only.
// https://github.com/moby/moby/issues/36503
func TestIpcPrivateVsReadonly(t *testing.T) {
	skip.If(t, os.Getuid() != 0, "skipping test that requires root")
	c := &container.Container{
		HostConfig: &containertypes.HostConfig{
			IpcMode:        containertypes.IPCModePrivate,
			ReadonlyRootfs: true,
		},
	}
	d := setupFakeDaemon(t, c)

	s, err := d.createSpec(context.TODO(), &configStore{}, c)
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
	skip.If(t, os.Getuid() != 0, "skipping test that requires root")
	c := &container.Container{
		Config: &containertypes.Config{
			Hostname:   "foobar",
			Domainname: "baz.cyphar.com",
		},
		HostConfig: &containertypes.HostConfig{
			NetworkMode: "bridge",
			Sysctls:     map[string]string{},
		},
	}
	d := setupFakeDaemon(t, c)

	// Ensure that the implicit sysctl is set correctly.
	s, err := d.createSpec(context.TODO(), &configStore{}, c)
	assert.NilError(t, err)
	assert.Equal(t, s.Hostname, "foobar")
	assert.Equal(t, s.Linux.Sysctl["kernel.domainname"], c.Config.Domainname)
	if sysctlExists("net.ipv4.ip_unprivileged_port_start") {
		assert.Equal(t, s.Linux.Sysctl["net.ipv4.ip_unprivileged_port_start"], "0")
	}
	if sysctlExists("net.ipv4.ping_group_range") {
		assert.Equal(t, s.Linux.Sysctl["net.ipv4.ping_group_range"], "0 2147483647")
	}

	// Set an explicit sysctl.
	c.HostConfig.Sysctls["kernel.domainname"] = "foobar.net"
	assert.Assert(t, c.HostConfig.Sysctls["kernel.domainname"] != c.Config.Domainname)
	c.HostConfig.Sysctls["net.ipv4.ip_unprivileged_port_start"] = "1024"

	s, err = d.createSpec(context.TODO(), &configStore{}, c)
	assert.NilError(t, err)
	assert.Equal(t, s.Hostname, "foobar")
	assert.Equal(t, s.Linux.Sysctl["kernel.domainname"], c.HostConfig.Sysctls["kernel.domainname"])
	assert.Equal(t, s.Linux.Sysctl["net.ipv4.ip_unprivileged_port_start"], c.HostConfig.Sysctls["net.ipv4.ip_unprivileged_port_start"])

	// Ensure the ping_group_range is not set on a daemon with user-namespaces enabled
	s, err = d.createSpec(context.TODO(), &configStore{Config: config.Config{RemappedRoot: "dummy:dummy"}}, c)
	assert.NilError(t, err)
	_, ok := s.Linux.Sysctl["net.ipv4.ping_group_range"]
	assert.Assert(t, !ok)

	// Ensure the ping_group_range is set on a container in "host" userns mode
	// on a daemon with user-namespaces enabled
	c.HostConfig.UsernsMode = "host"
	s, err = d.createSpec(context.TODO(), &configStore{Config: config.Config{RemappedRoot: "dummy:dummy"}}, c)
	assert.NilError(t, err)
	assert.Equal(t, s.Linux.Sysctl["net.ipv4.ping_group_range"], "0 2147483647")
}

// TestSysctlOverrideHost ensures that any implicit network sysctls are not set
// with host networking
func TestSysctlOverrideHost(t *testing.T) {
	skip.If(t, os.Getuid() != 0, "skipping test that requires root")
	c := &container.Container{
		Config: &containertypes.Config{},
		HostConfig: &containertypes.HostConfig{
			NetworkMode: "host",
			Sysctls:     map[string]string{},
		},
	}
	d := setupFakeDaemon(t, c)

	// Ensure that the implicit sysctl is not set
	s, err := d.createSpec(context.TODO(), &configStore{}, c)
	assert.NilError(t, err)
	assert.Equal(t, s.Linux.Sysctl["net.ipv4.ip_unprivileged_port_start"], "")
	assert.Equal(t, s.Linux.Sysctl["net.ipv4.ping_group_range"], "")

	// Set an explicit sysctl.
	c.HostConfig.Sysctls["net.ipv4.ip_unprivileged_port_start"] = "1024"

	s, err = d.createSpec(context.TODO(), &configStore{}, c)
	assert.NilError(t, err)
	assert.Equal(t, s.Linux.Sysctl["net.ipv4.ip_unprivileged_port_start"], c.HostConfig.Sysctls["net.ipv4.ip_unprivileged_port_start"])
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
