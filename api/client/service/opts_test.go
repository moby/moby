package service

import (
	"testing"
	"time"

	"github.com/docker/docker/pkg/testutil/assert"
	"github.com/docker/engine-api/types/swarm"
)

func TestMemBytesString(t *testing.T) {
	var mem memBytes = 1048576
	assert.Equal(t, mem.String(), "1 MiB")
}

func TestMemBytesSetAndValue(t *testing.T) {
	var mem memBytes
	assert.NilError(t, mem.Set("5kb"))
	assert.Equal(t, mem.Value(), int64(5120))
}

func TestNanoCPUsString(t *testing.T) {
	var cpus nanoCPUs = 6100000000
	assert.Equal(t, cpus.String(), "6.100")
}

func TestNanoCPUsSetAndValue(t *testing.T) {
	var cpus nanoCPUs
	assert.NilError(t, cpus.Set("0.35"))
	assert.Equal(t, cpus.Value(), int64(350000000))
}

func TestDurationOptString(t *testing.T) {
	dur := time.Duration(300 * 10e8)
	duration := DurationOpt{value: &dur}
	assert.Equal(t, duration.String(), "5m0s")
}

func TestDurationOptSetAndValue(t *testing.T) {
	var duration DurationOpt
	assert.NilError(t, duration.Set("300s"))
	assert.Equal(t, *duration.Value(), time.Duration(300*10e8))
}

func TestUint64OptString(t *testing.T) {
	value := uint64(2345678)
	opt := Uint64Opt{value: &value}
	assert.Equal(t, opt.String(), "2345678")

	opt = Uint64Opt{}
	assert.Equal(t, opt.String(), "none")
}

func TestUint64OptSetAndValue(t *testing.T) {
	var opt Uint64Opt
	assert.NilError(t, opt.Set("14445"))
	assert.Equal(t, *opt.Value(), uint64(14445))
}

func TestMountOptString(t *testing.T) {
	mount := MountOpt{
		values: []swarm.Mount{
			{
				Type:   swarm.MountType("BIND"),
				Source: "/home/path",
				Target: "/target",
			},
			{
				Type:   swarm.MountType("VOLUME"),
				Source: "foo",
				Target: "/target/foo",
			},
		},
	}
	expected := "BIND /home/path /target, VOLUME foo /target/foo"
	assert.Equal(t, mount.String(), expected)
}

func TestMountOptSetNoError(t *testing.T) {
	var mount MountOpt
	assert.NilError(t, mount.Set("type=bind,target=/target,source=/foo"))

	mounts := mount.Value()
	assert.Equal(t, len(mounts), 1)
	assert.Equal(t, mounts[0], swarm.Mount{
		Type:   swarm.MountType("BIND"),
		Source: "/foo",
		Target: "/target",
	})
}

func TestMountOptSetErrorNoType(t *testing.T) {
	var mount MountOpt
	assert.Error(t, mount.Set("target=/target,source=/foo"), "type is required")
}

func TestMountOptSetErrorNoTarget(t *testing.T) {
	var mount MountOpt
	assert.Error(t, mount.Set("type=VOLUME,source=/foo"), "target is required")
}

func TestMountOptSetErrorInvalidKey(t *testing.T) {
	var mount MountOpt
	assert.Error(t, mount.Set("type=VOLUME,bogus=foo"), "unexpected key 'bogus'")
}

func TestMountOptSetErrorInvalidField(t *testing.T) {
	var mount MountOpt
	assert.Error(t, mount.Set("type=VOLUME,bogus"), "invalid field 'bogus'")
}

func TestMountOptSetErrorInvalidWritable(t *testing.T) {
	var mount MountOpt
	assert.Error(t, mount.Set("type=VOLUME,writable=yes"), "invalid value for writable: yes")
}

func TestMountRaw(t *testing.T) {
	type c struct {
		raw      string
		expected swarm.Mount
	}
	cases := []c{
		{"/banana:/unicorn", swarm.Mount{Type: swarm.MountTypeBind, Source: "/banana", Target: "/unicorn", Writable: true}},
		{"/banana:/unicorn:ro", swarm.Mount{Type: swarm.MountTypeBind, Source: "/banana", Target: "/unicorn"}},
		{"/banana:/unicorn:ro,rprivate", swarm.Mount{Type: swarm.MountTypeBind, Source: "/banana", Target: "/unicorn", BindOptions: &swarm.BindOptions{Propagation: "rprivate"}}},
		{"/banana:/unicorn:rprivate", swarm.Mount{Type: swarm.MountTypeBind, Source: "/banana", Target: "/unicorn", Writable: true, BindOptions: &swarm.BindOptions{Propagation: "rprivate"}}},
		{"banana:/unicorn", swarm.Mount{Type: swarm.MountTypeVolume, Source: "banana", Target: "/unicorn", Writable: true, VolumeOptions: &swarm.VolumeOptions{Populate: true}}},
		{"banana:/unicorn:ro", swarm.Mount{Type: swarm.MountTypeVolume, Source: "banana", Target: "/unicorn", VolumeOptions: &swarm.VolumeOptions{Populate: true}}},
		{"banana:/unicorn:ro,nocopy", swarm.Mount{Type: swarm.MountTypeVolume, Source: "banana", Target: "/unicorn", VolumeOptions: &swarm.VolumeOptions{Populate: false}}},
		{"banana:/unicorn:nocopy", swarm.Mount{Type: swarm.MountTypeVolume, Source: "banana", Target: "/unicorn", Writable: true, VolumeOptions: &swarm.VolumeOptions{Populate: false}}},
		{"/unicorn", swarm.Mount{Type: swarm.MountTypeVolume, Target: "/unicorn", Writable: true, VolumeOptions: &swarm.VolumeOptions{Populate: true}}},
		{`c:\banana:/unicorn`, swarm.Mount{Type: swarm.MountTypeBind, Source: `c:\banana`, Target: "/unicorn", Writable: true}},
		{`c:\banana:/unicorn:ro`, swarm.Mount{Type: swarm.MountTypeBind, Source: `c:\banana`, Target: "/unicorn"}},
		{`c:\banana:c:\unicorn`, swarm.Mount{Type: swarm.MountTypeBind, Source: `c:\banana`, Target: `c:\unicorn`, Writable: true}},
		{`c:\banana:c:\unicorn:ro`, swarm.Mount{Type: swarm.MountTypeBind, Source: `c:\banana`, Target: `c:\unicorn`}},
		{`c:\banana:c:\unicorn:ro,rprivate`, swarm.Mount{Type: swarm.MountTypeBind, Source: `c:\banana`, Target: `c:\unicorn`, BindOptions: &swarm.BindOptions{Propagation: "rprivate"}}},
		{`c:\banana:c:\unicorn:rprivate`, swarm.Mount{Type: swarm.MountTypeBind, Source: `c:\banana`, Target: `c:\unicorn`, Writable: true, BindOptions: &swarm.BindOptions{Propagation: "rprivate"}}},
		{`c:\banana`, swarm.Mount{Type: swarm.MountTypeVolume, Target: `c:\banana`, Writable: true, VolumeOptions: &swarm.VolumeOptions{Populate: true}}},
	}

	for i, c := range cases {
		t.Logf("case %d - %s", i, c.raw)
		var m mountRawOpt
		if err := m.Set(c.raw); err != nil {
			t.Fatal(err)
		}
		if len(m.values) == 0 {
			t.Fatal("mounts not set")
		}
		actual := m.values[0]

		if actual.Source != c.expected.Source {
			t.Fatalf("expected source '%s' to match '%s'", actual.Source, c.expected.Source)
		}
		if actual.Target != c.expected.Target {
			t.Fatalf("expected destination '%s' to match '%s'", actual.Target, c.expected.Target)
		}
		if actual.Writable != c.expected.Writable {
			t.Fatalf("expected writable '%v' to match '%v'", actual.Writable, c.expected.Writable)
		}
		if c.expected.VolumeOptions != nil {
			if actual.VolumeOptions == nil {
				t.Fatal("unexpected nil VolumeOptions")
			}
			if actual.VolumeOptions.Populate != c.expected.VolumeOptions.Populate {
				t.Fatalf("expected volume-populate '%v' to match '%v'", actual.VolumeOptions.Populate, c.expected.VolumeOptions.Populate)
			}
		}
		if c.expected.BindOptions != nil {
			if actual.BindOptions == nil {
				t.Fatal("unexpected nil BindOptions")
			}
			if actual.BindOptions.Propagation != c.expected.BindOptions.Propagation {
				t.Fatalf("expected bind-propagation '%v' to match '%v'", actual.BindOptions.Propagation, c.expected.BindOptions.Propagation)
			}
		}
	}
}
