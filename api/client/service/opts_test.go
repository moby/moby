package service

import (
	"time"

	"github.com/docker/docker/pkg/testutil/assert"
	mounttypes "github.com/docker/engine-api/types/mount"
	"github.com/go-check/check"
)

func (s *DockerSuite) TestMemBytesString(c *check.C) {
	var mem memBytes = 1048576
	assert.Equal(c, mem.String(), "1 MiB")
}

func (s *DockerSuite) TestMemBytesSetAndValue(c *check.C) {
	var mem memBytes
	assert.NilError(c, mem.Set("5kb"))
	assert.Equal(c, mem.Value(), int64(5120))
}

func (s *DockerSuite) TestNanoCPUsString(c *check.C) {
	var cpus nanoCPUs = 6100000000
	assert.Equal(c, cpus.String(), "6.100")
}

func (s *DockerSuite) TestNanoCPUsSetAndValue(c *check.C) {
	var cpus nanoCPUs
	assert.NilError(c, cpus.Set("0.35"))
	assert.Equal(c, cpus.Value(), int64(350000000))
}

func (s *DockerSuite) TestDurationOptString(c *check.C) {
	dur := time.Duration(300 * 10e8)
	duration := DurationOpt{value: &dur}
	assert.Equal(c, duration.String(), "5m0s")
}

func (s *DockerSuite) TestDurationOptSetAndValue(c *check.C) {
	var duration DurationOpt
	assert.NilError(c, duration.Set("300s"))
	assert.Equal(c, *duration.Value(), time.Duration(300*10e8))
}

func (s *DockerSuite) TestUint64OptString(c *check.C) {
	value := uint64(2345678)
	opt := Uint64Opt{value: &value}
	assert.Equal(c, opt.String(), "2345678")

	opt = Uint64Opt{}
	assert.Equal(c, opt.String(), "none")
}

func (s *DockerSuite) TestUint64OptSetAndValue(c *check.C) {
	var opt Uint64Opt
	assert.NilError(c, opt.Set("14445"))
	assert.Equal(c, *opt.Value(), uint64(14445))
}

func (s *DockerSuite) TestMountOptString(c *check.C) {
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
	assert.Equal(c, mount.String(), expected)
}

func (s *DockerSuite) TestMountOptSetNoError(c *check.C) {
	for _, testcase := range []string{
		// tests several aliases that should have same result.
		"type=bind,target=/target,source=/source",
		"type=bind,src=/source,dst=/target",
		"type=bind,source=/source,dst=/target",
		"type=bind,src=/source,target=/target",
	} {
		var mount MountOpt

		assert.NilError(c, mount.Set(testcase))

		mounts := mount.Value()
		assert.Equal(c, len(mounts), 1)
		assert.Equal(c, mounts[0], mounttypes.Mount{
			Type:   mounttypes.TypeBind,
			Source: "/source",
			Target: "/target",
		})
	}
}

// TestMountOptDefaultType ensures that a mount without the type defaults to a
// volume mount.
func (s *DockerSuite) TestMountOptDefaultType(c *check.C) {
	var mount MountOpt
	assert.NilError(c, mount.Set("target=/target,source=/foo"))
	assert.Equal(c, mount.values[0].Type, mounttypes.TypeVolume)
}

func (s *DockerSuite) TestMountOptSetErrorNoTarget(c *check.C) {
	var mount MountOpt
	assert.Error(c, mount.Set("type=volume,source=/foo"), "target is required")
}

func (s *DockerSuite) TestMountOptSetErrorInvalidKey(c *check.C) {
	var mount MountOpt
	assert.Error(c, mount.Set("type=volume,bogus=foo"), "unexpected key 'bogus'")
}

func (s *DockerSuite) TestMountOptSetErrorInvalidField(c *check.C) {
	var mount MountOpt
	assert.Error(c, mount.Set("type=volume,bogus"), "invalid field 'bogus'")
}

func (s *DockerSuite) TestMountOptSetErrorInvalidReadOnly(c *check.C) {
	var mount MountOpt
	assert.Error(c, mount.Set("type=volume,readonly=no"), "invalid value for readonly: no")
	assert.Error(c, mount.Set("type=volume,readonly=invalid"), "invalid value for readonly: invalid")
}

func (s *DockerSuite) TestMountOptDefaultEnableReadOnly(c *check.C) {
	var m MountOpt
	assert.NilError(c, m.Set("type=bind,target=/foo,source=/foo"))
	assert.Equal(c, m.values[0].ReadOnly, false)

	m = MountOpt{}
	assert.NilError(c, m.Set("type=bind,target=/foo,source=/foo,readonly"))
	assert.Equal(c, m.values[0].ReadOnly, true)

	m = MountOpt{}
	assert.NilError(c, m.Set("type=bind,target=/foo,source=/foo,readonly=1"))
	assert.Equal(c, m.values[0].ReadOnly, true)

	m = MountOpt{}
	assert.NilError(c, m.Set("type=bind,target=/foo,source=/foo,readonly=0"))
	assert.Equal(c, m.values[0].ReadOnly, false)
}

func (s *DockerSuite) TestMountOptVolumeNoCopy(c *check.C) {
	var m MountOpt
	assert.Error(c, m.Set("type=volume,target=/foo,volume-nocopy"), "source is required")

	m = MountOpt{}
	assert.NilError(c, m.Set("type=volume,target=/foo,source=foo"))
	assert.Equal(c, m.values[0].VolumeOptions == nil, true)

	m = MountOpt{}
	assert.NilError(c, m.Set("type=volume,target=/foo,source=foo,volume-nocopy=true"))
	assert.Equal(c, m.values[0].VolumeOptions != nil, true)
	assert.Equal(c, m.values[0].VolumeOptions.NoCopy, true)

	m = MountOpt{}
	assert.NilError(c, m.Set("type=volume,target=/foo,source=foo,volume-nocopy"))
	assert.Equal(c, m.values[0].VolumeOptions != nil, true)
	assert.Equal(c, m.values[0].VolumeOptions.NoCopy, true)

	m = MountOpt{}
	assert.NilError(c, m.Set("type=volume,target=/foo,source=foo,volume-nocopy=1"))
	assert.Equal(c, m.values[0].VolumeOptions != nil, true)
	assert.Equal(c, m.values[0].VolumeOptions.NoCopy, true)
}

func (s *DockerSuite) TestMountOptTypeConflict(c *check.C) {
	var m MountOpt
	assert.Error(c, m.Set("type=bind,target=/foo,source=/foo,volume-nocopy=true"), "cannot mix")
	assert.Error(c, m.Set("type=volume,target=/foo,source=/foo,bind-propagation=rprivate"), "cannot mix")
}
