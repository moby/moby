package service

import (
	"time"

	mounttypes "github.com/docker/docker/api/types/mount"
	"github.com/go-check/check"
)

func (s *DockerSuite) TestMemBytesString(c *check.C) {
	var mem memBytes = 1048576
	c.Assert(mem.String(), check.Equals, "1 MiB")
}

func (s *DockerSuite) TestMemBytesSetAndValue(c *check.C) {
	var mem memBytes
	c.Assert(mem.Set("5kb"), check.IsNil)
	c.Assert(mem.Value(), check.Equals, int64(5120))
}

func (s *DockerSuite) TestNanoCPUsString(c *check.C) {
	var cpus nanoCPUs = 6100000000
	c.Assert(cpus.String(), check.Equals, "6.100")
}

func (s *DockerSuite) TestNanoCPUsSetAndValue(c *check.C) {
	var cpus nanoCPUs
	c.Assert(cpus.Set("0.35"), check.IsNil)
	c.Assert(cpus.Value(), check.Equals, int64(350000000))
}

func (s *DockerSuite) TestDurationOptString(c *check.C) {
	dur := time.Duration(300 * 10e8)
	duration := DurationOpt{value: &dur}
	c.Assert(duration.String(), check.Equals, "5m0s")
}

func (s *DockerSuite) TestDurationOptSetAndValue(c *check.C) {
	var duration DurationOpt
	c.Assert(duration.Set("300s"), check.IsNil)
	c.Assert(*duration.Value(), check.Equals, time.Duration(300*10e8))
}

func (s *DockerSuite) TestUint64OptString(c *check.C) {
	value := uint64(2345678)
	opt := Uint64Opt{value: &value}
	c.Assert(opt.String(), check.Equals, "2345678")

	opt = Uint64Opt{}
	c.Assert(opt.String(), check.Equals, "none")
}

func (s *DockerSuite) TestUint64OptSetAndValue(c *check.C) {
	var opt Uint64Opt
	c.Assert(opt.Set("14445"), check.IsNil)
	c.Assert(*opt.Value(), check.Equals, uint64(14445))
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
	c.Assert(mount.String(), check.Equals, expected)
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

		c.Assert(mount.Set(testcase), check.IsNil)

		mounts := mount.Value()
		c.Assert(len(mounts), check.Equals, 1)
		c.Assert(mounts[0], check.Equals, mounttypes.Mount{
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
	c.Assert(mount.Set("target=/target,source=/foo"), check.IsNil)
	c.Assert(mount.values[0].Type, check.Equals, mounttypes.TypeVolume)
}

func (s *DockerSuite) TestMountOptSetErrorNoTarget(c *check.C) {
	var mount MountOpt
	c.Assert(mount.Set("type=volume,source=/foo"), check.ErrorMatches, ".*target is required.*")
}

func (s *DockerSuite) TestMountOptSetErrorInvalidKey(c *check.C) {
	var mount MountOpt
	c.Assert(mount.Set("type=volume,bogus=foo"), check.ErrorMatches, ".*unexpected key 'bogus'.*")
}

func (s *DockerSuite) TestMountOptSetErrorInvalidField(c *check.C) {
	var mount MountOpt
	c.Assert(mount.Set("type=volume,bogus"), check.ErrorMatches, ".*invalid field 'bogus'.*")
}

func (s *DockerSuite) TestMountOptSetErrorInvalidReadOnly(c *check.C) {
	var mount MountOpt
	c.Assert(mount.Set("type=volume,readonly=no"), check.ErrorMatches, ".*invalid value for readonly: no.*")
	c.Assert(mount.Set("type=volume,readonly=invalid"), check.ErrorMatches, ".*invalid value for readonly: invalid.*")
}

func (s *DockerSuite) TestMountOptDefaultEnableReadOnly(c *check.C) {
	var m MountOpt
	c.Assert(m.Set("type=bind,target=/foo,source=/foo"), check.IsNil)
	c.Assert(m.values[0].ReadOnly, check.Equals, false)

	m = MountOpt{}
	c.Assert(m.Set("type=bind,target=/foo,source=/foo,readonly"), check.IsNil)
	c.Assert(m.values[0].ReadOnly, check.Equals, true)

	m = MountOpt{}
	c.Assert(m.Set("type=bind,target=/foo,source=/foo,readonly=1"), check.IsNil)
	c.Assert(m.values[0].ReadOnly, check.Equals, true)

	m = MountOpt{}
	c.Assert(m.Set("type=bind,target=/foo,source=/foo,readonly=0"), check.IsNil)
	c.Assert(m.values[0].ReadOnly, check.Equals, false)
}

func (s *DockerSuite) TestMountOptVolumeNoCopy(c *check.C) {
	var m MountOpt
	c.Assert(m.Set("type=volume,target=/foo,volume-nocopy"), check.ErrorMatches, ".*source is required.*")

	m = MountOpt{}
	c.Assert(m.Set("type=volume,target=/foo,source=foo"), check.IsNil)
	c.Assert(m.values[0].VolumeOptions == nil, check.Equals, true)

	m = MountOpt{}
	c.Assert(m.Set("type=volume,target=/foo,source=foo,volume-nocopy=true"), check.IsNil)
	c.Assert(m.values[0].VolumeOptions != nil, check.Equals, true)
	c.Assert(m.values[0].VolumeOptions.NoCopy, check.Equals, true)

	m = MountOpt{}
	c.Assert(m.Set("type=volume,target=/foo,source=foo,volume-nocopy"), check.IsNil)
	c.Assert(m.values[0].VolumeOptions != nil, check.Equals, true)
	c.Assert(m.values[0].VolumeOptions.NoCopy, check.Equals, true)

	m = MountOpt{}
	c.Assert(m.Set("type=volume,target=/foo,source=foo,volume-nocopy=1"), check.IsNil)
	c.Assert(m.values[0].VolumeOptions != nil, check.Equals, true)
	c.Assert(m.values[0].VolumeOptions.NoCopy, check.Equals, true)
}

func (s *DockerSuite) TestMountOptTypeConflict(c *check.C) {
	var m MountOpt
	c.Assert(m.Set("type=bind,target=/foo,source=/foo,volume-nocopy=true"), check.ErrorMatches, ".*cannot mix.*")
	c.Assert(m.Set("type=volume,target=/foo,source=/foo,bind-propagation=rprivate"), check.ErrorMatches, ".*cannot mix.*")
}
