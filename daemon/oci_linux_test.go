package daemon // import "github.com/docker/docker/daemon"

import (
	"testing"

	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/container"
	"github.com/docker/docker/daemon/config"
	"github.com/docker/docker/oci"
	"github.com/docker/docker/pkg/idtools"
	"github.com/gotestyourself/gotestyourself/assert"
	is "github.com/gotestyourself/gotestyourself/assert/cmp"
)

// TestTmpfsDevShmNoDupMount checks that a user-specified /dev/shm tmpfs
// mount (as in "docker run --tmpfs /dev/shm:rw,size=NNN") does not result
// in "Duplicate mount point" error from the engine.
// https://github.com/moby/moby/issues/35455
func TestTmpfsDevShmNoDupMount(t *testing.T) {
	d := Daemon{
		// some empty structs to avoid getting a panic
		// caused by a null pointer dereference
		idMappings:  &idtools.IDMappings{},
		configStore: &config.Config{},
	}
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

	// Mimick the code flow of daemon.createSpec(), enough to reproduce the issue
	ms, err := d.setupMounts(c)
	assert.Check(t, err)

	ms = append(ms, c.IpcMounts()...)

	tmpfsMounts, err := c.TmpfsMounts()
	assert.Check(t, err)
	ms = append(ms, tmpfsMounts...)

	s := oci.DefaultSpec()
	err = setMounts(&d, &s, c, ms)
	assert.Check(t, err)
}

// TestIpcPrivateVsReadonly checks that in case of IpcMode: private
// and ReadonlyRootfs: true (as in "docker run --ipc private --read-only")
// the resulting /dev/shm mount is NOT made read-only.
// https://github.com/moby/moby/issues/36503
func TestIpcPrivateVsReadonly(t *testing.T) {
	d := Daemon{
		// some empty structs to avoid getting a panic
		// caused by a null pointer dereference
		idMappings:  &idtools.IDMappings{},
		configStore: &config.Config{},
	}
	c := &container.Container{
		HostConfig: &containertypes.HostConfig{
			IpcMode:        containertypes.IpcMode("private"),
			ReadonlyRootfs: true,
		},
	}

	// We can't call createSpec() so mimick the minimal part
	// of its code flow, just enough to reproduce the issue.
	ms, err := d.setupMounts(c)
	assert.Check(t, err)

	s := oci.DefaultSpec()
	s.Root.Readonly = c.HostConfig.ReadonlyRootfs

	err = setMounts(&d, &s, c, ms)
	assert.Check(t, err)

	// Find the /dev/shm mount in ms, check it does not have ro
	for _, m := range s.Mounts {
		if m.Destination != "/dev/shm" {
			continue
		}
		assert.Check(t, is.Equal(false, inSlice(m.Options, "ro")))
	}
}
