package opts

import (
	"os"
	"testing"

	mounttypes "github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/pkg/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMountOptString(t *testing.T) {
	mount := MountOpt{
		values: []mounttypes.Mount{
			{
				Type:   mounttypes.TypeBind,
				Source: "/home/path",
				Target: "/target",
			},
			{
				Type:   mounttypes.TypeVolume,
				Source: "foo",
				Target: "/target/foo",
			},
		},
	}
	expected := "bind /home/path /target, volume foo /target/foo"
	assert.Equal(t, expected, mount.String())
}

func TestMountOptSetBindNoErrorBind(t *testing.T) {
	for _, testcase := range []string{
		// tests several aliases that should have same result.
		"type=bind,target=/target,source=/source",
		"type=bind,src=/source,dst=/target",
		"type=bind,source=/source,dst=/target",
		"type=bind,src=/source,target=/target",
	} {
		var mount MountOpt

		assert.NoError(t, mount.Set(testcase))

		mounts := mount.Value()
		require.Len(t, mounts, 1)
		assert.Equal(t, mounttypes.Mount{
			Type:   mounttypes.TypeBind,
			Source: "/source",
			Target: "/target",
		}, mounts[0])
	}
}

func TestMountOptSetVolumeNoError(t *testing.T) {
	for _, testcase := range []string{
		// tests several aliases that should have same result.
		"type=volume,target=/target,source=/source",
		"type=volume,src=/source,dst=/target",
		"type=volume,source=/source,dst=/target",
		"type=volume,src=/source,target=/target",
	} {
		var mount MountOpt

		assert.NoError(t, mount.Set(testcase))

		mounts := mount.Value()
		require.Len(t, mounts, 1)
		assert.Equal(t, mounttypes.Mount{
			Type:   mounttypes.TypeVolume,
			Source: "/source",
			Target: "/target",
		}, mounts[0])
	}
}

// TestMountOptDefaultType ensures that a mount without the type defaults to a
// volume mount.
func TestMountOptDefaultType(t *testing.T) {
	var mount MountOpt
	assert.NoError(t, mount.Set("target=/target,source=/foo"))
	assert.Equal(t, mounttypes.TypeVolume, mount.values[0].Type)
}

func TestMountOptSetErrorNoTarget(t *testing.T) {
	var mount MountOpt
	assert.EqualError(t, mount.Set("type=volume,source=/foo"), "target is required")
}

func TestMountOptSetErrorInvalidKey(t *testing.T) {
	var mount MountOpt
	assert.EqualError(t, mount.Set("type=volume,bogus=foo"), "unexpected key 'bogus' in 'bogus=foo'")
}

func TestMountOptSetErrorInvalidField(t *testing.T) {
	var mount MountOpt
	assert.EqualError(t, mount.Set("type=volume,bogus"), "invalid field 'bogus' must be a key=value pair")
}

func TestMountOptSetErrorInvalidReadOnly(t *testing.T) {
	var mount MountOpt
	assert.EqualError(t, mount.Set("type=volume,readonly=no"), "invalid value for readonly: no")
	assert.EqualError(t, mount.Set("type=volume,readonly=invalid"), "invalid value for readonly: invalid")
}

func TestMountOptDefaultEnableReadOnly(t *testing.T) {
	var m MountOpt
	assert.NoError(t, m.Set("type=bind,target=/foo,source=/foo"))
	assert.False(t, m.values[0].ReadOnly)

	m = MountOpt{}
	assert.NoError(t, m.Set("type=bind,target=/foo,source=/foo,readonly"))
	assert.True(t, m.values[0].ReadOnly)

	m = MountOpt{}
	assert.NoError(t, m.Set("type=bind,target=/foo,source=/foo,readonly=1"))
	assert.True(t, m.values[0].ReadOnly)

	m = MountOpt{}
	assert.NoError(t, m.Set("type=bind,target=/foo,source=/foo,readonly=true"))
	assert.True(t, m.values[0].ReadOnly)

	m = MountOpt{}
	assert.NoError(t, m.Set("type=bind,target=/foo,source=/foo,readonly=0"))
	assert.False(t, m.values[0].ReadOnly)
}

func TestMountOptVolumeNoCopy(t *testing.T) {
	var m MountOpt
	assert.NoError(t, m.Set("type=volume,target=/foo,volume-nocopy"))
	assert.Equal(t, "", m.values[0].Source)

	m = MountOpt{}
	assert.NoError(t, m.Set("type=volume,target=/foo,source=foo"))
	assert.True(t, m.values[0].VolumeOptions == nil)

	m = MountOpt{}
	assert.NoError(t, m.Set("type=volume,target=/foo,source=foo,volume-nocopy=true"))
	assert.True(t, m.values[0].VolumeOptions != nil)
	assert.True(t, m.values[0].VolumeOptions.NoCopy)

	m = MountOpt{}
	assert.NoError(t, m.Set("type=volume,target=/foo,source=foo,volume-nocopy"))
	assert.True(t, m.values[0].VolumeOptions != nil)
	assert.True(t, m.values[0].VolumeOptions.NoCopy)

	m = MountOpt{}
	assert.NoError(t, m.Set("type=volume,target=/foo,source=foo,volume-nocopy=1"))
	assert.True(t, m.values[0].VolumeOptions != nil)
	assert.True(t, m.values[0].VolumeOptions.NoCopy)
}

func TestMountOptTypeConflict(t *testing.T) {
	var m MountOpt
	testutil.ErrorContains(t, m.Set("type=bind,target=/foo,source=/foo,volume-nocopy=true"), "cannot mix")
	testutil.ErrorContains(t, m.Set("type=volume,target=/foo,source=/foo,bind-propagation=rprivate"), "cannot mix")
}

func TestMountOptSetTmpfsNoError(t *testing.T) {
	for _, testcase := range []string{
		// tests several aliases that should have same result.
		"type=tmpfs,target=/target,tmpfs-size=1m,tmpfs-mode=0700",
		"type=tmpfs,target=/target,tmpfs-size=1MB,tmpfs-mode=700",
	} {
		var mount MountOpt

		assert.NoError(t, mount.Set(testcase))

		mounts := mount.Value()
		require.Len(t, mounts, 1)
		assert.Equal(t, mounttypes.Mount{
			Type:   mounttypes.TypeTmpfs,
			Target: "/target",
			TmpfsOptions: &mounttypes.TmpfsOptions{
				SizeBytes: 1024 * 1024, // not 1000 * 1000
				Mode:      os.FileMode(0700),
			},
		}, mounts[0])
	}
}

func TestMountOptSetTmpfsError(t *testing.T) {
	var m MountOpt
	testutil.ErrorContains(t, m.Set("type=tmpfs,target=/foo,tmpfs-size=foo"), "invalid value for tmpfs-size")
	testutil.ErrorContains(t, m.Set("type=tmpfs,target=/foo,tmpfs-mode=foo"), "invalid value for tmpfs-mode")
	testutil.ErrorContains(t, m.Set("type=tmpfs"), "target is required")
}
