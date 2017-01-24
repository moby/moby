package opts

import (
	"os"
	"testing"

	mounttypes "github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/pkg/testutil/assert"
)

func TestVolumeOptSetBindNoErrorBind(t *testing.T) {
	for _, testcase := range []string{
		// tests several aliases that should have same result.
		"type=bind,target=/target,source=/source",
		"type=bind,src=/source,dst=/target",
		"type=bind,source=/source,dst=/target",
		"type=bind,src=/source,target=/target",
	} {
		var mount VolumeOpt

		assert.NilError(t, mount.Set(testcase))

		mounts := mount.LongValue()
		assert.Equal(t, len(mounts), 1)
		assert.Equal(t, mounts[0], mounttypes.Mount{
			Type:   mounttypes.TypeBind,
			Source: "/source",
			Target: "/target",
		})
	}
}

func TestVolumeOptSetVolumeNoError(t *testing.T) {
	for _, testcase := range []string{
		// tests several aliases that should have same result.
		"type=volume,target=/target,source=/source",
		"type=volume,src=/source,dst=/target",
		"type=volume,source=/source,dst=/target",
		"type=volume,src=/source,target=/target",
	} {
		var mount VolumeOpt

		assert.NilError(t, mount.Set(testcase))

		mounts := mount.LongValue()
		assert.Equal(t, len(mounts), 1)
		assert.Equal(t, mounts[0], mounttypes.Mount{
			Type:   mounttypes.TypeVolume,
			Source: "/source",
			Target: "/target",
		})
	}
}

// TestVolumeOptDefaultType ensures that a mount without the type defaults to a
// volume mount.
func TestVolumeOptDefaultType(t *testing.T) {
	var mount VolumeOpt
	assert.NilError(t, mount.Set("target=/target,source=/foo"))
	assert.Equal(t, mount.longs[0].Type, mounttypes.TypeVolume)
}

func TestVolumeOptSetErrorNoTarget(t *testing.T) {
	var mount VolumeOpt
	assert.Error(t, mount.Set("type=volume,source=/foo"), "target is required")
}

func TestVolumeOptSetErrorInvalidKey(t *testing.T) {
	var mount VolumeOpt
	assert.Error(t, mount.Set("type=volume,bogus=foo"), "unexpected key 'bogus'")
}

func TestVolumeOptSetErrorInvalidField(t *testing.T) {
	var mount VolumeOpt
	assert.Error(t, mount.Set("type=volume,bogus"), "invalid field 'bogus'")
}

func TestVolumeOptSetErrorInvalidReadOnly(t *testing.T) {
	var mount VolumeOpt
	assert.Error(t, mount.Set("type=volume,readonly=no"), "invalid value for readonly: no")
	assert.Error(t, mount.Set("type=volume,readonly=invalid"), "invalid value for readonly: invalid")
}

func TestVolumeOptDefaultEnableReadOnly(t *testing.T) {
	var m VolumeOpt
	assert.NilError(t, m.Set("type=bind,target=/foo,source=/foo"))
	assert.Equal(t, m.longs[0].ReadOnly, false)

	m = VolumeOpt{}
	assert.NilError(t, m.Set("type=bind,target=/foo,source=/foo,readonly"))
	assert.Equal(t, m.longs[0].ReadOnly, true)

	m = VolumeOpt{}
	assert.NilError(t, m.Set("type=bind,target=/foo,source=/foo,readonly=1"))
	assert.Equal(t, m.longs[0].ReadOnly, true)

	m = VolumeOpt{}
	assert.NilError(t, m.Set("type=bind,target=/foo,source=/foo,readonly=true"))
	assert.Equal(t, m.longs[0].ReadOnly, true)

	m = VolumeOpt{}
	assert.NilError(t, m.Set("type=bind,target=/foo,source=/foo,readonly=0"))
	assert.Equal(t, m.longs[0].ReadOnly, false)
}

func TestVolumeOptVolumeNoCopy(t *testing.T) {
	var m VolumeOpt
	assert.NilError(t, m.Set("type=volume,target=/foo,volume-nocopy"))
	assert.Equal(t, m.longs[0].Source, "")

	m = VolumeOpt{}
	assert.NilError(t, m.Set("type=volume,target=/foo,source=foo"))
	assert.Equal(t, m.longs[0].VolumeOptions == nil, true)

	m = VolumeOpt{}
	assert.NilError(t, m.Set("type=volume,target=/foo,source=foo,volume-nocopy=true"))
	assert.Equal(t, m.longs[0].VolumeOptions != nil, true)
	assert.Equal(t, m.longs[0].VolumeOptions.NoCopy, true)

	m = VolumeOpt{}
	assert.NilError(t, m.Set("type=volume,target=/foo,source=foo,volume-nocopy"))
	assert.Equal(t, m.longs[0].VolumeOptions != nil, true)
	assert.Equal(t, m.longs[0].VolumeOptions.NoCopy, true)

	m = VolumeOpt{}
	assert.NilError(t, m.Set("type=volume,target=/foo,source=foo,volume-nocopy=1"))
	assert.Equal(t, m.longs[0].VolumeOptions != nil, true)
	assert.Equal(t, m.longs[0].VolumeOptions.NoCopy, true)
}

func TestVolumeOptTypeConflict(t *testing.T) {
	var m VolumeOpt
	assert.Error(t, m.Set("type=bind,target=/foo,source=/foo,volume-nocopy=true"), "cannot mix")
	assert.Error(t, m.Set("type=volume,target=/foo,source=/foo,bind-propagation=rprivate"), "cannot mix")
}

func TestVolumeOptSetTmpfsNoError(t *testing.T) {
	for _, testcase := range []string{
		// tests several aliases that should have same result.
		"type=tmpfs,target=/target,tmpfs-size=1m,tmpfs-mode=0700",
		"type=tmpfs,target=/target,tmpfs-size=1MB,tmpfs-mode=700",
	} {
		var mount VolumeOpt

		assert.NilError(t, mount.Set(testcase))

		mounts := mount.LongValue()
		assert.Equal(t, len(mounts), 1)
		assert.DeepEqual(t, mounts[0], mounttypes.Mount{
			Type:   mounttypes.TypeTmpfs,
			Target: "/target",
			TmpfsOptions: &mounttypes.TmpfsOptions{
				SizeBytes: 1024 * 1024, // not 1000 * 1000
				Mode:      os.FileMode(0700),
			},
		})
	}
}

func TestVolumeOptSetTmpfsError(t *testing.T) {
	var m VolumeOpt
	assert.Error(t, m.Set("type=tmpfs,target=/foo,tmpfs-size=foo"), "invalid value for tmpfs-size")
	assert.Error(t, m.Set("type=tmpfs,target=/foo,tmpfs-mode=foo"), "invalid value for tmpfs-mode")
	assert.Error(t, m.Set("type=tmpfs"), "target is required")
}
